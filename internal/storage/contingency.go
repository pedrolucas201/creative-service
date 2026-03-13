package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type ContingencyCandidate struct {
	CampaignID   string    `json:"campaign_id"`
	AdID         string    `json:"ad_id"`
	AdStatus     string    `json:"ad_status"`
	ErrorCode    string    `json:"error_code,omitempty"`
	ErrorSummary string    `json:"error_summary,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	SyncedAt     time.Time `json:"synced_at"`
}

type ContingencyIncident struct {
	IncidentUUID      string          `json:"incident_uuid"`
	SourceCampaignID  string          `json:"source_campaign_id"`
	SourceAdAccountID string          `json:"source_ad_account_id"`
	TriggerType       string          `json:"trigger_type"`
	ReasonCode        string          `json:"reason_code"`
	ReasonDetail      string          `json:"reason_detail,omitempty"`
	Evidence          json.RawMessage `json:"evidence,omitempty"`
	Status            string          `json:"status"`
	AttemptCount      int             `json:"attempt_count"`
	OpenedAt          time.Time       `json:"opened_at"`
	ClosedAt          *time.Time      `json:"closed_at,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type ContingencyNode struct {
	NodeUUID     string     `json:"node_uuid"`
	BMUUID       string     `json:"bm_uuid"`
	AdAccountID  string     `json:"ad_account_id"`
	NodeName     string     `json:"node_name"`
	Priority     int        `json:"priority"`
	Weight       int        `json:"weight"`
	IsActive     bool       `json:"is_active"`
	CooldownTill *time.Time `json:"cooldown_until,omitempty"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type ContingencyNodeView struct {
	ContingencyNode
	ClientUUID    string `json:"client_uuid"`
	AdAccountName string `json:"ad_account_name"`
	BMID          string `json:"bm_id,omitempty"`
}

type ContingencyRoute struct {
	RouteUUID         string    `json:"route_uuid"`
	SourceAdAccountID string    `json:"source_ad_account_id"`
	TargetNodeUUID    string    `json:"target_node_uuid"`
	OrderIndex        int       `json:"order_index"`
	IsActive          bool      `json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type ContingencyRouteView struct {
	ContingencyRoute
	TargetNode        ContingencyNodeView `json:"target_node"`
	SourceAccountName string              `json:"source_account_name,omitempty"`
}

type ContingencyExecution struct {
	ExecutionUUID  string     `json:"execution_uuid"`
	IncidentUUID   string     `json:"incident_uuid"`
	Attempt        int        `json:"attempt"`
	TargetNodeUUID *string    `json:"target_node_uuid,omitempty"`
	Status         string     `json:"status"`
	ErrorCode      string     `json:"error_code,omitempty"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type CreateContingencyIncidentInput struct {
	SourceCampaignID  string
	SourceAdAccountID string
	TriggerType       string
	ReasonCode        string
	ReasonDetail      string
	Evidence          json.RawMessage
}

type CompleteContingencyExecutionInput struct {
	IncidentUUID    string
	ExecutionUUID   string
	ExecutionStatus string
	IncidentStatus  string
	TargetNodeUUID  *string
	ErrorCode       string
	ErrorMessage    string
}

type CloseContingencyIncidentInput struct {
	IncidentUUID string
	FinalStatus  string
	ReasonDetail string
}

type UpsertContingencyNodeInput struct {
	AdAccountID   string
	NodeName      string
	Priority      int
	Weight        int
	IsActive      bool
	CooldownUntil *time.Time
}

type UpsertContingencyRouteInput struct {
	SourceAdAccountID string
	TargetNodeUUID    string
	OrderIndex        int
	IsActive          bool
}

type CreateEntitySwitchMapInput struct {
	IncidentUUID     string
	SourceCampaignID string
	TargetCampaignID string
	SourceAdSetIDs   []string
	TargetAdSetIDs   []string
	SourceAdIDs      []string
	TargetAdIDs      []string
}

type EntitySwitchMap struct {
	SwitchUUID       string          `json:"switch_uuid"`
	IncidentUUID     string          `json:"incident_uuid"`
	SourceCampaignID string          `json:"source_campaign_id"`
	TargetCampaignID string          `json:"target_campaign_id,omitempty"`
	SourceAdSetIDs   json.RawMessage `json:"source_adset_ids"`
	TargetAdSetIDs   json.RawMessage `json:"target_adset_ids"`
	SourceAdIDs      json.RawMessage `json:"source_ad_ids"`
	TargetAdIDs      json.RawMessage `json:"target_ad_ids"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

func normalizeContingencyTriggerType(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func validateContingencyTriggerType(v string) bool {
	switch normalizeContingencyTriggerType(v) {
	case "webhook", "polling", "manual":
		return true
	default:
		return false
	}
}

func validateContingencyCloseStatus(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "closed", "manual_required":
		return true
	default:
		return false
	}
}

func (s *Store) ListContingencyCandidatesByAdAccount(
	ctx context.Context,
	adAccountID string,
	limit int,
) ([]ContingencyCandidate, error) {
	adAccountID = strings.TrimSpace(adAccountID)
	if adAccountID == "" {
		return nil, errors.New("ad_account_id is required")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := s.DB.Query(ctx, `
		SELECT
			COALESCE(
				NULLIF(raw_payload->>'campaign_id', ''),
				NULLIF(raw_payload->'graph'->>'campaign_id', ''),
				''
			) AS campaign_id,
			entity_id AS ad_id,
			COALESCE(status, '') AS ad_status,
			COALESCE(
				NULLIF(raw_payload->'webhook'->'value'->>'error_code', ''),
				NULLIF(raw_payload->>'error_code', ''),
				''
			) AS error_code,
			COALESCE(
				NULLIF(raw_payload->'webhook'->'value'->>'error_summary', ''),
				NULLIF(raw_payload->>'error_summary', ''),
				''
			) AS error_summary,
			COALESCE(
				NULLIF(raw_payload->'webhook'->'value'->>'error_message', ''),
				NULLIF(raw_payload->>'error_message', ''),
				''
			) AS error_message,
			synced_at
		FROM entity_status_cache
		WHERE ad_account_id = $1
		  AND entity_type = 'ad'
		  AND status IN ('DISAPPROVED', 'WITH_ISSUES')
		ORDER BY synced_at DESC
		LIMIT $2
	`, adAccountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ContingencyCandidate, 0, limit)
	for rows.Next() {
		var item ContingencyCandidate
		if err := rows.Scan(
			&item.CampaignID,
			&item.AdID,
			&item.AdStatus,
			&item.ErrorCode,
			&item.ErrorSummary,
			&item.ErrorMessage,
			&item.SyncedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) CreateOrGetOpenContingencyIncident(
	ctx context.Context,
	in CreateContingencyIncidentInput,
) (ContingencyIncident, bool, error) {
	in.SourceCampaignID = strings.TrimSpace(in.SourceCampaignID)
	in.SourceAdAccountID = strings.TrimSpace(in.SourceAdAccountID)
	in.TriggerType = normalizeContingencyTriggerType(in.TriggerType)
	in.ReasonCode = strings.TrimSpace(in.ReasonCode)
	in.ReasonDetail = strings.TrimSpace(in.ReasonDetail)

	if in.SourceCampaignID == "" {
		return ContingencyIncident{}, false, errors.New("source_campaign_id is required")
	}
	if in.SourceAdAccountID == "" {
		return ContingencyIncident{}, false, errors.New("source_ad_account_id is required")
	}
	if !validateContingencyTriggerType(in.TriggerType) {
		return ContingencyIncident{}, false, fmt.Errorf("invalid trigger_type: %q", in.TriggerType)
	}
	if in.ReasonCode == "" {
		return ContingencyIncident{}, false, errors.New("reason_code is required")
	}

	evidence := in.Evidence
	if len(evidence) == 0 {
		evidence = json.RawMessage(`{}`)
	}

	var item ContingencyIncident
	err := s.DB.QueryRow(ctx, `
		INSERT INTO contingency_incidents(
			source_campaign_id,
			source_ad_account_id,
			trigger_type,
			reason_code,
			reason_detail,
			evidence,
			status,
			attempt_count,
			opened_at
		)
		VALUES($1, $2, $3, $4, $5, $6, 'detected', 0, now())
		RETURNING
			incident_uuid::text,
			source_campaign_id,
			source_ad_account_id,
			trigger_type,
			reason_code,
			COALESCE(reason_detail, ''),
			evidence,
			status,
			attempt_count,
			opened_at,
			closed_at,
			created_at,
			updated_at
	`, in.SourceCampaignID, in.SourceAdAccountID, in.TriggerType, in.ReasonCode, in.ReasonDetail, evidence).Scan(
		&item.IncidentUUID,
		&item.SourceCampaignID,
		&item.SourceAdAccountID,
		&item.TriggerType,
		&item.ReasonCode,
		&item.ReasonDetail,
		&item.Evidence,
		&item.Status,
		&item.AttemptCount,
		&item.OpenedAt,
		&item.ClosedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err == nil {
		return item, true, nil
	}
	if !isUniqueViolation(err) {
		return ContingencyIncident{}, false, err
	}

	existing, getErr := s.GetOpenContingencyIncidentByCampaign(
		ctx,
		in.SourceCampaignID,
		in.SourceAdAccountID,
	)
	if getErr != nil {
		return ContingencyIncident{}, false, getErr
	}
	return existing, false, nil
}

func (s *Store) GetOpenContingencyIncidentByCampaign(
	ctx context.Context,
	sourceCampaignID, sourceAdAccountID string,
) (ContingencyIncident, error) {
	var item ContingencyIncident
	err := s.DB.QueryRow(ctx, `
		SELECT
			incident_uuid::text,
			source_campaign_id,
			source_ad_account_id,
			trigger_type,
			reason_code,
			COALESCE(reason_detail, ''),
			evidence,
			status,
			attempt_count,
			opened_at,
			closed_at,
			created_at,
			updated_at
		FROM contingency_incidents
		WHERE source_campaign_id = $1
		  AND source_ad_account_id = $2
		  AND status IN ('detected', 'queued', 'executing')
		ORDER BY opened_at DESC
		LIMIT 1
	`, sourceCampaignID, sourceAdAccountID).Scan(
		&item.IncidentUUID,
		&item.SourceCampaignID,
		&item.SourceAdAccountID,
		&item.TriggerType,
		&item.ReasonCode,
		&item.ReasonDetail,
		&item.Evidence,
		&item.Status,
		&item.AttemptCount,
		&item.OpenedAt,
		&item.ClosedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	return item, err
}

func (s *Store) ListContingencyIncidentsByAdAccount(
	ctx context.Context,
	adAccountID string,
	onlyOpen bool,
	limit int,
) ([]ContingencyIncident, error) {
	adAccountID = strings.TrimSpace(adAccountID)
	if adAccountID == "" {
		return nil, errors.New("ad_account_id is required")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	query := `
		SELECT
			incident_uuid::text,
			source_campaign_id,
			source_ad_account_id,
			trigger_type,
			reason_code,
			COALESCE(reason_detail, ''),
			evidence,
			status,
			attempt_count,
			opened_at,
			closed_at,
			created_at,
			updated_at
		FROM contingency_incidents
		WHERE source_ad_account_id = $1
	`
	args := []any{adAccountID}

	if onlyOpen {
		query += " AND status IN ('detected', 'queued', 'executing')"
	}

	query += " ORDER BY opened_at DESC LIMIT $2"
	args = append(args, limit)

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ContingencyIncident, 0, limit)
	for rows.Next() {
		var item ContingencyIncident
		if err := rows.Scan(
			&item.IncidentUUID,
			&item.SourceCampaignID,
			&item.SourceAdAccountID,
			&item.TriggerType,
			&item.ReasonCode,
			&item.ReasonDetail,
			&item.Evidence,
			&item.Status,
			&item.AttemptCount,
			&item.OpenedAt,
			&item.ClosedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) GetContingencyIncidentByUUID(
	ctx context.Context,
	incidentUUID string,
) (ContingencyIncident, error) {
	incidentUUID = strings.TrimSpace(incidentUUID)
	if incidentUUID == "" {
		return ContingencyIncident{}, errors.New("incident_uuid is required")
	}

	var item ContingencyIncident
	err := s.DB.QueryRow(ctx, `
		SELECT
			incident_uuid::text,
			source_campaign_id,
			source_ad_account_id,
			trigger_type,
			reason_code,
			COALESCE(reason_detail, ''),
			evidence,
			status,
			attempt_count,
			opened_at,
			closed_at,
			created_at,
			updated_at
		FROM contingency_incidents
		WHERE incident_uuid = $1::uuid
		LIMIT 1
	`, incidentUUID).Scan(
		&item.IncidentUUID,
		&item.SourceCampaignID,
		&item.SourceAdAccountID,
		&item.TriggerType,
		&item.ReasonCode,
		&item.ReasonDetail,
		&item.Evidence,
		&item.Status,
		&item.AttemptCount,
		&item.OpenedAt,
		&item.ClosedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	return item, err
}

func (s *Store) CloseContingencyIncident(
	ctx context.Context,
	in CloseContingencyIncidentInput,
) (ContingencyIncident, error) {
	in.IncidentUUID = strings.TrimSpace(in.IncidentUUID)
	in.FinalStatus = strings.ToLower(strings.TrimSpace(in.FinalStatus))
	in.ReasonDetail = strings.TrimSpace(in.ReasonDetail)

	if in.IncidentUUID == "" {
		return ContingencyIncident{}, errors.New("incident_uuid is required")
	}
	if in.FinalStatus == "" {
		in.FinalStatus = "closed"
	}
	if !validateContingencyCloseStatus(in.FinalStatus) {
		return ContingencyIncident{}, errors.New("invalid contingency close status")
	}

	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return ContingencyIncident{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentStatus string
	err = tx.QueryRow(ctx, `
		SELECT status
		FROM contingency_incidents
		WHERE incident_uuid = $1::uuid
		FOR UPDATE
	`, in.IncidentUUID).Scan(&currentStatus)
	if err != nil {
		return ContingencyIncident{}, err
	}

	switch strings.ToLower(strings.TrimSpace(currentStatus)) {
	case "closed", "switched", "rolled_back":
		return ContingencyIncident{}, errors.New("contingency incident already closed")
	}

	var runningExecutionID string
	checkErr := tx.QueryRow(ctx, `
		SELECT execution_uuid::text
		FROM contingency_executions
		WHERE incident_uuid = $1::uuid
		  AND status = 'running'
		ORDER BY created_at DESC
		LIMIT 1
	`, in.IncidentUUID).Scan(&runningExecutionID)
	if checkErr == nil {
		return ContingencyIncident{}, errors.New("contingency incident execution in progress")
	}
	if checkErr != nil && !errors.Is(checkErr, pgx.ErrNoRows) {
		return ContingencyIncident{}, checkErr
	}

	var incident ContingencyIncident
	err = tx.QueryRow(ctx, `
		UPDATE contingency_incidents
		SET
			status = $2,
			reason_detail = CASE
				WHEN NULLIF($3, '') IS NOT NULL THEN $3
				ELSE reason_detail
			END,
			closed_at = COALESCE(closed_at, now()),
			updated_at = now()
		WHERE incident_uuid = $1::uuid
		RETURNING
			incident_uuid::text,
			source_campaign_id,
			source_ad_account_id,
			trigger_type,
			reason_code,
			COALESCE(reason_detail, ''),
			evidence,
			status,
			attempt_count,
			opened_at,
			closed_at,
			created_at,
			updated_at
	`, in.IncidentUUID, in.FinalStatus, in.ReasonDetail).Scan(
		&incident.IncidentUUID,
		&incident.SourceCampaignID,
		&incident.SourceAdAccountID,
		&incident.TriggerType,
		&incident.ReasonCode,
		&incident.ReasonDetail,
		&incident.Evidence,
		&incident.Status,
		&incident.AttemptCount,
		&incident.OpenedAt,
		&incident.ClosedAt,
		&incident.CreatedAt,
		&incident.UpdatedAt,
	)
	if err != nil {
		return ContingencyIncident{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return ContingencyIncident{}, err
	}

	return incident, nil
}

func (s *Store) ListContingencyAvailableAccountsBySource(
	ctx context.Context,
	sourceAdAccountID string,
) ([]AdAccount, error) {
	sourceAdAccountID = strings.TrimSpace(sourceAdAccountID)
	if sourceAdAccountID == "" {
		return nil, errors.New("source_ad_account_id is required")
	}

	rows, err := s.DB.Query(ctx, `
		SELECT
			aa.ad_account_id,
			aa.client_uuid,
			aa.bm_uuid,
			COALESCE(bm.bm_id, '') AS bm_id,
			COALESCE(bm.is_active, FALSE) AS bm_is_active,
			aa.ad_account_name,
			aa.page_id,
			aa.token_ref,
			aa.is_active,
			aa.deleted_at,
			aa.created_at,
			aa.updated_at
		FROM ad_accounts src
		JOIN ad_accounts aa
		  ON aa.bm_uuid = src.bm_uuid
		LEFT JOIN business_managers bm
		  ON bm.bm_uuid = aa.bm_uuid
		WHERE src.ad_account_id = $1
		  AND src.bm_uuid IS NOT NULL
		  AND aa.deleted_at IS NULL
		  AND aa.ad_account_id <> $1
		ORDER BY aa.ad_account_name, aa.ad_account_id
	`, sourceAdAccountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AdAccount, 0, 8)
	for rows.Next() {
		var item AdAccount
		if err := rows.Scan(
			&item.AdAccountID,
			&item.ClientUUID,
			&item.BMUUID,
			&item.BMID,
			&item.BMIsActive,
			&item.AdAccountName,
			&item.PageID,
			&item.TokenRef,
			&item.IsActive,
			&item.DeletedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ListContingencyNodesBySource(
	ctx context.Context,
	sourceAdAccountID string,
) ([]ContingencyNodeView, error) {
	sourceAdAccountID = strings.TrimSpace(sourceAdAccountID)
	if sourceAdAccountID == "" {
		return nil, errors.New("source_ad_account_id is required")
	}

	rows, err := s.DB.Query(ctx, `
		SELECT
			cn.node_uuid::text,
			cn.bm_uuid::text,
			cn.ad_account_id,
			cn.node_name,
			cn.priority,
			cn.weight,
			cn.is_active,
			cn.cooldown_until,
			cn.last_used_at,
			cn.created_at,
			cn.updated_at,
			aa.client_uuid,
			aa.ad_account_name,
			COALESCE(bm.bm_id, '') AS bm_id
		FROM ad_accounts src
		JOIN contingency_nodes cn
		  ON cn.bm_uuid = src.bm_uuid
		JOIN ad_accounts aa
		  ON aa.ad_account_id = cn.ad_account_id
		LEFT JOIN business_managers bm
		  ON bm.bm_uuid = cn.bm_uuid
		WHERE src.ad_account_id = $1
		  AND src.bm_uuid IS NOT NULL
		  AND aa.deleted_at IS NULL
		ORDER BY cn.priority ASC, aa.ad_account_name ASC, aa.ad_account_id ASC
	`, sourceAdAccountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ContingencyNodeView, 0, 8)
	for rows.Next() {
		var item ContingencyNodeView
		if err := rows.Scan(
			&item.NodeUUID,
			&item.BMUUID,
			&item.AdAccountID,
			&item.NodeName,
			&item.Priority,
			&item.Weight,
			&item.IsActive,
			&item.CooldownTill,
			&item.LastUsedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.ClientUUID,
			&item.AdAccountName,
			&item.BMID,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ListContingencyRoutesBySource(
	ctx context.Context,
	sourceAdAccountID string,
) ([]ContingencyRouteView, error) {
	sourceAdAccountID = strings.TrimSpace(sourceAdAccountID)
	if sourceAdAccountID == "" {
		return nil, errors.New("source_ad_account_id is required")
	}

	rows, err := s.DB.Query(ctx, `
		SELECT
			cr.route_uuid::text,
			cr.source_ad_account_id,
			cr.target_node_uuid::text,
			cr.order_index,
			cr.is_active,
			cr.created_at,
			cr.updated_at,
			cn.node_uuid::text,
			cn.bm_uuid::text,
			cn.ad_account_id,
			cn.node_name,
			cn.priority,
			cn.weight,
			cn.is_active,
			cn.cooldown_until,
			cn.last_used_at,
			cn.created_at,
			cn.updated_at,
			dst.client_uuid,
			dst.ad_account_name,
			COALESCE(bm.bm_id, '') AS bm_id,
			COALESCE(src.ad_account_name, '') AS source_account_name
		FROM contingency_routes cr
		JOIN contingency_nodes cn
		  ON cn.node_uuid = cr.target_node_uuid
		JOIN ad_accounts dst
		  ON dst.ad_account_id = cn.ad_account_id
		JOIN ad_accounts src
		  ON src.ad_account_id = cr.source_ad_account_id
		LEFT JOIN business_managers bm
		  ON bm.bm_uuid = cn.bm_uuid
		WHERE cr.source_ad_account_id = $1
		  AND dst.deleted_at IS NULL
		ORDER BY cr.order_index ASC, cn.priority ASC, dst.ad_account_name ASC
	`, sourceAdAccountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ContingencyRouteView, 0, 8)
	for rows.Next() {
		var item ContingencyRouteView
		if err := rows.Scan(
			&item.RouteUUID,
			&item.SourceAdAccountID,
			&item.TargetNodeUUID,
			&item.OrderIndex,
			&item.IsActive,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.TargetNode.NodeUUID,
			&item.TargetNode.BMUUID,
			&item.TargetNode.AdAccountID,
			&item.TargetNode.NodeName,
			&item.TargetNode.Priority,
			&item.TargetNode.Weight,
			&item.TargetNode.IsActive,
			&item.TargetNode.CooldownTill,
			&item.TargetNode.LastUsedAt,
			&item.TargetNode.CreatedAt,
			&item.TargetNode.UpdatedAt,
			&item.TargetNode.ClientUUID,
			&item.TargetNode.AdAccountName,
			&item.TargetNode.BMID,
			&item.SourceAccountName,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) UpsertContingencyNode(
	ctx context.Context,
	in UpsertContingencyNodeInput,
) (ContingencyNode, error) {
	in.AdAccountID = strings.TrimSpace(in.AdAccountID)
	in.NodeName = strings.TrimSpace(in.NodeName)
	if in.AdAccountID == "" {
		return ContingencyNode{}, errors.New("ad_account_id is required")
	}
	if in.NodeName == "" {
		return ContingencyNode{}, errors.New("node_name is required")
	}
	if in.Priority <= 0 {
		in.Priority = 100
	}
	if in.Weight <= 0 {
		in.Weight = 100
	}

	var item ContingencyNode
	err := s.DB.QueryRow(ctx, `
		INSERT INTO contingency_nodes(
			bm_uuid,
			ad_account_id,
			node_name,
			priority,
			weight,
			is_active,
			cooldown_until
		)
		SELECT
			aa.bm_uuid,
			aa.ad_account_id,
			$2,
			$3,
			$4,
			$5,
			$6
		FROM ad_accounts aa
		WHERE aa.ad_account_id = $1
		  AND aa.deleted_at IS NULL
		  AND aa.bm_uuid IS NOT NULL
		ON CONFLICT (ad_account_id)
		DO UPDATE SET
			node_name = EXCLUDED.node_name,
			priority = EXCLUDED.priority,
			weight = EXCLUDED.weight,
			is_active = EXCLUDED.is_active,
			cooldown_until = EXCLUDED.cooldown_until,
			updated_at = now()
		RETURNING
			node_uuid::text,
			bm_uuid::text,
			ad_account_id,
			node_name,
			priority,
			weight,
			is_active,
			cooldown_until,
			last_used_at,
			created_at,
			updated_at
	`, in.AdAccountID, in.NodeName, in.Priority, in.Weight, in.IsActive, in.CooldownUntil).Scan(
		&item.NodeUUID,
		&item.BMUUID,
		&item.AdAccountID,
		&item.NodeName,
		&item.Priority,
		&item.Weight,
		&item.IsActive,
		&item.CooldownTill,
		&item.LastUsedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return ContingencyNode{}, err
	}
	return item, nil
}

func (s *Store) UpsertContingencyRoute(
	ctx context.Context,
	in UpsertContingencyRouteInput,
) (ContingencyRouteView, error) {
	in.SourceAdAccountID = strings.TrimSpace(in.SourceAdAccountID)
	in.TargetNodeUUID = strings.TrimSpace(in.TargetNodeUUID)
	if in.SourceAdAccountID == "" {
		return ContingencyRouteView{}, errors.New("source_ad_account_id is required")
	}
	if in.TargetNodeUUID == "" {
		return ContingencyRouteView{}, errors.New("target_node_uuid is required")
	}
	if in.OrderIndex <= 0 {
		in.OrderIndex = 1
	}

	var item ContingencyRouteView
	err := s.DB.QueryRow(ctx, `
		WITH upserted AS (
			INSERT INTO contingency_routes(
				source_ad_account_id,
				target_node_uuid,
				order_index,
				is_active
			)
			SELECT
				src.ad_account_id,
				cn.node_uuid,
				$3,
				$4
			FROM ad_accounts src
			JOIN contingency_nodes cn
			  ON cn.node_uuid = $2::uuid
			JOIN ad_accounts dst
			  ON dst.ad_account_id = cn.ad_account_id
			WHERE src.ad_account_id = $1
			  AND src.deleted_at IS NULL
			  AND src.bm_uuid IS NOT NULL
			  AND dst.deleted_at IS NULL
			  AND dst.bm_uuid = src.bm_uuid
			  AND cn.ad_account_id <> src.ad_account_id
			ON CONFLICT (source_ad_account_id, target_node_uuid)
			DO UPDATE SET
				order_index = EXCLUDED.order_index,
				is_active = EXCLUDED.is_active,
				updated_at = now()
			RETURNING
				route_uuid::text,
				source_ad_account_id,
				target_node_uuid::text,
				order_index,
				is_active,
				created_at,
				updated_at
		)
		SELECT
			u.route_uuid,
			u.source_ad_account_id,
			u.target_node_uuid,
			u.order_index,
			u.is_active,
			u.created_at,
			u.updated_at,
			cn.node_uuid::text,
			cn.bm_uuid::text,
			cn.ad_account_id,
			cn.node_name,
			cn.priority,
			cn.weight,
			cn.is_active,
			cn.cooldown_until,
			cn.last_used_at,
			cn.created_at,
			cn.updated_at,
			dst.client_uuid,
			dst.ad_account_name,
			COALESCE(bm.bm_id, '') AS bm_id,
			COALESCE(src.ad_account_name, '') AS source_account_name
		FROM upserted u
		JOIN contingency_nodes cn
		  ON cn.node_uuid::text = u.target_node_uuid
		JOIN ad_accounts dst
		  ON dst.ad_account_id = cn.ad_account_id
		JOIN ad_accounts src
		  ON src.ad_account_id = u.source_ad_account_id
		LEFT JOIN business_managers bm
		  ON bm.bm_uuid = cn.bm_uuid
	`, in.SourceAdAccountID, in.TargetNodeUUID, in.OrderIndex, in.IsActive).Scan(
		&item.RouteUUID,
		&item.SourceAdAccountID,
		&item.TargetNodeUUID,
		&item.OrderIndex,
		&item.IsActive,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.TargetNode.NodeUUID,
		&item.TargetNode.BMUUID,
		&item.TargetNode.AdAccountID,
		&item.TargetNode.NodeName,
		&item.TargetNode.Priority,
		&item.TargetNode.Weight,
		&item.TargetNode.IsActive,
		&item.TargetNode.CooldownTill,
		&item.TargetNode.LastUsedAt,
		&item.TargetNode.CreatedAt,
		&item.TargetNode.UpdatedAt,
		&item.TargetNode.ClientUUID,
		&item.TargetNode.AdAccountName,
		&item.TargetNode.BMID,
		&item.SourceAccountName,
	)
	if err != nil {
		return ContingencyRouteView{}, err
	}
	return item, nil
}

func (s *Store) DeleteContingencyRoute(
	ctx context.Context,
	routeUUID string,
) (bool, error) {
	routeUUID = strings.TrimSpace(routeUUID)
	if routeUUID == "" {
		return false, errors.New("route_uuid is required")
	}

	result, err := s.DB.Exec(ctx, `
		DELETE FROM contingency_routes
		WHERE route_uuid = $1::uuid
	`, routeUUID)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() > 0, nil
}

func (s *Store) GetContingencyRouteByUUID(
	ctx context.Context,
	routeUUID string,
) (ContingencyRouteView, error) {
	routeUUID = strings.TrimSpace(routeUUID)
	if routeUUID == "" {
		return ContingencyRouteView{}, errors.New("route_uuid is required")
	}

	var item ContingencyRouteView
	err := s.DB.QueryRow(ctx, `
		SELECT
			cr.route_uuid::text,
			cr.source_ad_account_id,
			cr.target_node_uuid::text,
			cr.order_index,
			cr.is_active,
			cr.created_at,
			cr.updated_at,
			cn.node_uuid::text,
			cn.bm_uuid::text,
			cn.ad_account_id,
			cn.node_name,
			cn.priority,
			cn.weight,
			cn.is_active,
			cn.cooldown_until,
			cn.last_used_at,
			cn.created_at,
			cn.updated_at,
			dst.client_uuid,
			dst.ad_account_name,
			COALESCE(bm.bm_id, '') AS bm_id,
			COALESCE(src.ad_account_name, '') AS source_account_name
		FROM contingency_routes cr
		JOIN contingency_nodes cn
		  ON cn.node_uuid = cr.target_node_uuid
		JOIN ad_accounts dst
		  ON dst.ad_account_id = cn.ad_account_id
		JOIN ad_accounts src
		  ON src.ad_account_id = cr.source_ad_account_id
		LEFT JOIN business_managers bm
		  ON bm.bm_uuid = cn.bm_uuid
		WHERE cr.route_uuid = $1::uuid
		  AND dst.deleted_at IS NULL
		LIMIT 1
	`, routeUUID).Scan(
		&item.RouteUUID,
		&item.SourceAdAccountID,
		&item.TargetNodeUUID,
		&item.OrderIndex,
		&item.IsActive,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.TargetNode.NodeUUID,
		&item.TargetNode.BMUUID,
		&item.TargetNode.AdAccountID,
		&item.TargetNode.NodeName,
		&item.TargetNode.Priority,
		&item.TargetNode.Weight,
		&item.TargetNode.IsActive,
		&item.TargetNode.CooldownTill,
		&item.TargetNode.LastUsedAt,
		&item.TargetNode.CreatedAt,
		&item.TargetNode.UpdatedAt,
		&item.TargetNode.ClientUUID,
		&item.TargetNode.AdAccountName,
		&item.TargetNode.BMID,
		&item.SourceAccountName,
	)
	if err != nil {
		return ContingencyRouteView{}, err
	}
	return item, nil
}

func (s *Store) StartContingencyExecution(
	ctx context.Context,
	incidentUUID string,
) (ContingencyIncident, ContingencyExecution, error) {
	incidentUUID = strings.TrimSpace(incidentUUID)
	if incidentUUID == "" {
		return ContingencyIncident{}, ContingencyExecution{}, errors.New("incident_uuid is required")
	}

	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return ContingencyIncident{}, ContingencyExecution{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var incident ContingencyIncident
	err = tx.QueryRow(ctx, `
		SELECT
			incident_uuid::text,
			source_campaign_id,
			source_ad_account_id,
			trigger_type,
			reason_code,
			COALESCE(reason_detail, ''),
			evidence,
			status,
			attempt_count,
			opened_at,
			closed_at,
			created_at,
			updated_at
		FROM contingency_incidents
		WHERE incident_uuid = $1::uuid
		FOR UPDATE
	`, incidentUUID).Scan(
		&incident.IncidentUUID,
		&incident.SourceCampaignID,
		&incident.SourceAdAccountID,
		&incident.TriggerType,
		&incident.ReasonCode,
		&incident.ReasonDetail,
		&incident.Evidence,
		&incident.Status,
		&incident.AttemptCount,
		&incident.OpenedAt,
		&incident.ClosedAt,
		&incident.CreatedAt,
		&incident.UpdatedAt,
	)
	if err != nil {
		return ContingencyIncident{}, ContingencyExecution{}, err
	}

	switch incident.Status {
	case "switched", "rolled_back", "closed":
		return ContingencyIncident{}, ContingencyExecution{}, errors.New("contingency incident is not executable in current status")
	}

	var runningExecutionID string
	checkErr := tx.QueryRow(ctx, `
		SELECT execution_uuid::text
		FROM contingency_executions
		WHERE incident_uuid = $1::uuid
		  AND status = 'running'
		ORDER BY created_at DESC
		LIMIT 1
	`, incidentUUID).Scan(&runningExecutionID)
	if checkErr == nil {
		return ContingencyIncident{}, ContingencyExecution{}, errors.New("contingency execution already in progress")
	}
	if checkErr != nil && !errors.Is(checkErr, pgx.ErrNoRows) {
		return ContingencyIncident{}, ContingencyExecution{}, checkErr
	}

	attempt := incident.AttemptCount + 1

	var execution ContingencyExecution
	err = tx.QueryRow(ctx, `
		INSERT INTO contingency_executions(
			incident_uuid,
			attempt,
			status,
			started_at
		)
		VALUES($1::uuid, $2, 'running', now())
		RETURNING
			execution_uuid::text,
			incident_uuid::text,
			attempt,
			target_node_uuid::text,
			status,
			COALESCE(error_code, ''),
			COALESCE(error_message, ''),
			started_at,
			finished_at,
			created_at,
			updated_at
	`, incidentUUID, attempt).Scan(
		&execution.ExecutionUUID,
		&execution.IncidentUUID,
		&execution.Attempt,
		&execution.TargetNodeUUID,
		&execution.Status,
		&execution.ErrorCode,
		&execution.ErrorMessage,
		&execution.StartedAt,
		&execution.FinishedAt,
		&execution.CreatedAt,
		&execution.UpdatedAt,
	)
	if err != nil {
		return ContingencyIncident{}, ContingencyExecution{}, err
	}

	err = tx.QueryRow(ctx, `
		UPDATE contingency_incidents
		SET
			status = 'executing',
			attempt_count = $2,
			updated_at = now()
		WHERE incident_uuid = $1::uuid
		RETURNING
			incident_uuid::text,
			source_campaign_id,
			source_ad_account_id,
			trigger_type,
			reason_code,
			COALESCE(reason_detail, ''),
			evidence,
			status,
			attempt_count,
			opened_at,
			closed_at,
			created_at,
			updated_at
	`, incidentUUID, attempt).Scan(
		&incident.IncidentUUID,
		&incident.SourceCampaignID,
		&incident.SourceAdAccountID,
		&incident.TriggerType,
		&incident.ReasonCode,
		&incident.ReasonDetail,
		&incident.Evidence,
		&incident.Status,
		&incident.AttemptCount,
		&incident.OpenedAt,
		&incident.ClosedAt,
		&incident.CreatedAt,
		&incident.UpdatedAt,
	)
	if err != nil {
		return ContingencyIncident{}, ContingencyExecution{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return ContingencyIncident{}, ContingencyExecution{}, err
	}

	return incident, execution, nil
}

func (s *Store) PickContingencyTargetNode(
	ctx context.Context,
	sourceAdAccountID string,
) (ContingencyNode, bool, error) {
	sourceAdAccountID = strings.TrimSpace(sourceAdAccountID)
	if sourceAdAccountID == "" {
		return ContingencyNode{}, false, errors.New("source_ad_account_id is required")
	}

	scanNode := func(query string, args ...any) (ContingencyNode, bool, error) {
		var node ContingencyNode
		err := s.DB.QueryRow(ctx, query, args...).Scan(
			&node.NodeUUID,
			&node.BMUUID,
			&node.AdAccountID,
			&node.NodeName,
			&node.Priority,
			&node.Weight,
			&node.IsActive,
			&node.CooldownTill,
			&node.LastUsedAt,
			&node.CreatedAt,
			&node.UpdatedAt,
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ContingencyNode{}, false, nil
			}
			return ContingencyNode{}, false, err
		}
		return node, true, nil
	}

	byRoute, found, err := scanNode(`
		SELECT
			cn.node_uuid::text,
			cn.bm_uuid::text,
			cn.ad_account_id,
			cn.node_name,
			cn.priority,
			cn.weight,
			cn.is_active,
			cn.cooldown_until,
			cn.last_used_at,
			cn.created_at,
			cn.updated_at
		FROM contingency_routes cr
		JOIN contingency_nodes cn ON cn.node_uuid = cr.target_node_uuid
		JOIN ad_accounts src ON src.ad_account_id = cr.source_ad_account_id
		JOIN ad_accounts dst ON dst.ad_account_id = cn.ad_account_id
		WHERE cr.source_ad_account_id = $1
		  AND cr.is_active = TRUE
		  AND cn.is_active = TRUE
		  AND cn.ad_account_id <> $1
		  AND src.bm_uuid IS NOT NULL
		  AND dst.bm_uuid = src.bm_uuid
		  AND dst.deleted_at IS NULL
		  AND (cn.cooldown_until IS NULL OR cn.cooldown_until <= now())
		ORDER BY
		  cr.order_index ASC,
		  cn.priority ASC,
		  cn.last_used_at ASC NULLS FIRST
		LIMIT 1
	`, sourceAdAccountID)
	if err != nil {
		return ContingencyNode{}, false, err
	}
	if found {
		return byRoute, true, nil
	}

	fallback, found, err := scanNode(`
		SELECT
			cn.node_uuid::text,
			cn.bm_uuid::text,
			cn.ad_account_id,
			cn.node_name,
			cn.priority,
			cn.weight,
			cn.is_active,
			cn.cooldown_until,
			cn.last_used_at,
			cn.created_at,
			cn.updated_at
		FROM contingency_nodes cn
		JOIN ad_accounts src ON src.ad_account_id = $1
		JOIN ad_accounts dst ON dst.ad_account_id = cn.ad_account_id
		WHERE cn.is_active = TRUE
		  AND cn.ad_account_id <> $1
		  AND src.bm_uuid IS NOT NULL
		  AND dst.bm_uuid = src.bm_uuid
		  AND dst.deleted_at IS NULL
		  AND (cn.cooldown_until IS NULL OR cn.cooldown_until <= now())
		ORDER BY
		  cn.priority ASC,
		  cn.last_used_at ASC NULLS FIRST
		LIMIT 1
	`, sourceAdAccountID)
	if err != nil {
		return ContingencyNode{}, false, err
	}
	if found {
		return fallback, true, nil
	}

	return ContingencyNode{}, false, nil
}

func (s *Store) CompleteContingencyExecution(
	ctx context.Context,
	in CompleteContingencyExecutionInput,
) (ContingencyIncident, ContingencyExecution, error) {
	in.IncidentUUID = strings.TrimSpace(in.IncidentUUID)
	in.ExecutionUUID = strings.TrimSpace(in.ExecutionUUID)
	in.ExecutionStatus = strings.ToLower(strings.TrimSpace(in.ExecutionStatus))
	in.IncidentStatus = strings.ToLower(strings.TrimSpace(in.IncidentStatus))
	in.ErrorCode = strings.TrimSpace(in.ErrorCode)
	in.ErrorMessage = strings.TrimSpace(in.ErrorMessage)

	if in.IncidentUUID == "" {
		return ContingencyIncident{}, ContingencyExecution{}, errors.New("incident_uuid is required")
	}
	if in.ExecutionUUID == "" {
		return ContingencyIncident{}, ContingencyExecution{}, errors.New("execution_uuid is required")
	}
	if in.ExecutionStatus == "" {
		return ContingencyIncident{}, ContingencyExecution{}, errors.New("execution_status is required")
	}
	if in.IncidentStatus == "" {
		return ContingencyIncident{}, ContingencyExecution{}, errors.New("incident_status is required")
	}

	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return ContingencyIncident{}, ContingencyExecution{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var execution ContingencyExecution
	err = tx.QueryRow(ctx, `
		UPDATE contingency_executions
		SET
			status = $3,
			target_node_uuid = COALESCE($4::uuid, target_node_uuid),
			error_code = NULLIF($5, ''),
			error_message = NULLIF($6, ''),
			finished_at = now(),
			updated_at = now()
		WHERE execution_uuid = $1::uuid
		  AND incident_uuid = $2::uuid
		RETURNING
			execution_uuid::text,
			incident_uuid::text,
			attempt,
			target_node_uuid::text,
			status,
			COALESCE(error_code, ''),
			COALESCE(error_message, ''),
			started_at,
			finished_at,
			created_at,
			updated_at
	`, in.ExecutionUUID, in.IncidentUUID, in.ExecutionStatus, in.TargetNodeUUID, in.ErrorCode, in.ErrorMessage).Scan(
		&execution.ExecutionUUID,
		&execution.IncidentUUID,
		&execution.Attempt,
		&execution.TargetNodeUUID,
		&execution.Status,
		&execution.ErrorCode,
		&execution.ErrorMessage,
		&execution.StartedAt,
		&execution.FinishedAt,
		&execution.CreatedAt,
		&execution.UpdatedAt,
	)
	if err != nil {
		return ContingencyIncident{}, ContingencyExecution{}, err
	}

	var incident ContingencyIncident
	err = tx.QueryRow(ctx, `
		UPDATE contingency_incidents
		SET
			status = $2,
			closed_at = CASE
				WHEN $2 IN ('switched', 'rolled_back', 'closed', 'manual_required')
					THEN COALESCE(closed_at, now())
				ELSE NULL
			END,
			updated_at = now()
		WHERE incident_uuid = $1::uuid
		RETURNING
			incident_uuid::text,
			source_campaign_id,
			source_ad_account_id,
			trigger_type,
			reason_code,
			COALESCE(reason_detail, ''),
			evidence,
			status,
			attempt_count,
			opened_at,
			closed_at,
			created_at,
			updated_at
	`, in.IncidentUUID, in.IncidentStatus).Scan(
		&incident.IncidentUUID,
		&incident.SourceCampaignID,
		&incident.SourceAdAccountID,
		&incident.TriggerType,
		&incident.ReasonCode,
		&incident.ReasonDetail,
		&incident.Evidence,
		&incident.Status,
		&incident.AttemptCount,
		&incident.OpenedAt,
		&incident.ClosedAt,
		&incident.CreatedAt,
		&incident.UpdatedAt,
	)
	if err != nil {
		return ContingencyIncident{}, ContingencyExecution{}, err
	}

	if in.TargetNodeUUID != nil && strings.TrimSpace(*in.TargetNodeUUID) != "" {
		if _, err := tx.Exec(ctx, `
			UPDATE contingency_nodes
			SET last_used_at = now(), updated_at = now()
			WHERE node_uuid = $1::uuid
		`, *in.TargetNodeUUID); err != nil {
			return ContingencyIncident{}, ContingencyExecution{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return ContingencyIncident{}, ContingencyExecution{}, err
	}

	return incident, execution, nil
}

func (s *Store) ListContingencyExecutionsByIncident(
	ctx context.Context,
	incidentUUID string,
	limit int,
) ([]ContingencyExecution, error) {
	incidentUUID = strings.TrimSpace(incidentUUID)
	if incidentUUID == "" {
		return nil, errors.New("incident_uuid is required")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := s.DB.Query(ctx, `
		SELECT
			execution_uuid::text,
			incident_uuid::text,
			attempt,
			target_node_uuid::text,
			status,
			COALESCE(error_code, ''),
			COALESCE(error_message, ''),
			started_at,
			finished_at,
			created_at,
			updated_at
		FROM contingency_executions
		WHERE incident_uuid = $1::uuid
		ORDER BY created_at DESC
		LIMIT $2
	`, incidentUUID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ContingencyExecution, 0, limit)
	for rows.Next() {
		var item ContingencyExecution
		if err := rows.Scan(
			&item.ExecutionUUID,
			&item.IncidentUUID,
			&item.Attempt,
			&item.TargetNodeUUID,
			&item.Status,
			&item.ErrorCode,
			&item.ErrorMessage,
			&item.StartedAt,
			&item.FinishedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func encodeStringSliceAsJSON(value []string) (json.RawMessage, error) {
	if len(value) == 0 {
		return json.RawMessage(`[]`), nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *Store) CreateEntitySwitchMap(
	ctx context.Context,
	in CreateEntitySwitchMapInput,
) (EntitySwitchMap, error) {
	in.IncidentUUID = strings.TrimSpace(in.IncidentUUID)
	in.SourceCampaignID = strings.TrimSpace(in.SourceCampaignID)
	in.TargetCampaignID = strings.TrimSpace(in.TargetCampaignID)

	if in.IncidentUUID == "" {
		return EntitySwitchMap{}, errors.New("incident_uuid is required")
	}
	if in.SourceCampaignID == "" {
		return EntitySwitchMap{}, errors.New("source_campaign_id is required")
	}

	sourceAdSetIDs, err := encodeStringSliceAsJSON(in.SourceAdSetIDs)
	if err != nil {
		return EntitySwitchMap{}, err
	}
	targetAdSetIDs, err := encodeStringSliceAsJSON(in.TargetAdSetIDs)
	if err != nil {
		return EntitySwitchMap{}, err
	}
	sourceAdIDs, err := encodeStringSliceAsJSON(in.SourceAdIDs)
	if err != nil {
		return EntitySwitchMap{}, err
	}
	targetAdIDs, err := encodeStringSliceAsJSON(in.TargetAdIDs)
	if err != nil {
		return EntitySwitchMap{}, err
	}

	var out EntitySwitchMap
	err = s.DB.QueryRow(ctx, `
		INSERT INTO entity_switch_map(
			incident_uuid,
			source_campaign_id,
			target_campaign_id,
			source_adset_ids,
			target_adset_ids,
			source_ad_ids,
			target_ad_ids
		)
		VALUES($1::uuid, $2, NULLIF($3, ''), $4, $5, $6, $7)
		RETURNING
			switch_uuid::text,
			incident_uuid::text,
			source_campaign_id,
			COALESCE(target_campaign_id, ''),
			source_adset_ids,
			target_adset_ids,
			source_ad_ids,
			target_ad_ids,
			created_at,
			updated_at
	`,
		in.IncidentUUID,
		in.SourceCampaignID,
		in.TargetCampaignID,
		sourceAdSetIDs,
		targetAdSetIDs,
		sourceAdIDs,
		targetAdIDs,
	).Scan(
		&out.SwitchUUID,
		&out.IncidentUUID,
		&out.SourceCampaignID,
		&out.TargetCampaignID,
		&out.SourceAdSetIDs,
		&out.TargetAdSetIDs,
		&out.SourceAdIDs,
		&out.TargetAdIDs,
		&out.CreatedAt,
		&out.UpdatedAt,
	)
	if err != nil {
		return EntitySwitchMap{}, err
	}

	return out, nil
}

func (s *Store) ListEntitySwitchMapByIncident(
	ctx context.Context,
	incidentUUID string,
	limit int,
) ([]EntitySwitchMap, error) {
	incidentUUID = strings.TrimSpace(incidentUUID)
	if incidentUUID == "" {
		return nil, errors.New("incident_uuid is required")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := s.DB.Query(ctx, `
		SELECT
			switch_uuid::text,
			incident_uuid::text,
			source_campaign_id,
			COALESCE(target_campaign_id, ''),
			source_adset_ids,
			target_adset_ids,
			source_ad_ids,
			target_ad_ids,
			created_at,
			updated_at
		FROM entity_switch_map
		WHERE incident_uuid = $1::uuid
		ORDER BY created_at DESC
		LIMIT $2
	`, incidentUUID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]EntitySwitchMap, 0, limit)
	for rows.Next() {
		var item EntitySwitchMap
		if err := rows.Scan(
			&item.SwitchUUID,
			&item.IncidentUUID,
			&item.SourceCampaignID,
			&item.TargetCampaignID,
			&item.SourceAdSetIDs,
			&item.TargetAdSetIDs,
			&item.SourceAdIDs,
			&item.TargetAdIDs,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
