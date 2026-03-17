package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ DB *pgxpool.Pool }

func New(db *pgxpool.Pool) *Store { return &Store{DB: db} }

type Client struct {
	ClientUUID string     `json:"client_uuid"`
	ClientID   string     `json:"client_id"`
	Name       string     `json:"name"`
	Email      *string    `json:"email,omitempty"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type AdAccount struct {
	AdAccountID   string     `json:"ad_account_id"`
	ClientUUID    string     `json:"client_uuid"`
	BMUUID        *string    `json:"bm_uuid,omitempty"`
	BMID          string     `json:"bm_id,omitempty"`
	BMIsActive    bool       `json:"bm_is_active"`
	AdAccountName string     `json:"ad_account_name"`
	PageID        string     `json:"page_id"`
	TokenRef      string     `json:"token_ref"`
	IsActive      bool       `json:"is_active"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (s *Store) GetClient(ctx context.Context, clientID string) (Client, error) {
	var c Client
	err := s.DB.QueryRow(ctx, `
		SELECT client_uuid, client_id, name, email, deleted_at, created_at, updated_at
		FROM clients 
		WHERE client_id = $1 AND deleted_at IS NULL
	`, clientID).Scan(&c.ClientUUID, &c.ClientID, &c.Name, &c.Email, &c.DeletedAt, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (s *Store) GetClientByUUID(ctx context.Context, clientUUID string) (Client, error) {
	var c Client
	err := s.DB.QueryRow(ctx, `
		SELECT client_uuid, client_id, name, email, deleted_at, created_at, updated_at
		FROM clients 
		WHERE client_uuid = $1 AND deleted_at IS NULL
	`, clientUUID).Scan(&c.ClientUUID, &c.ClientID, &c.Name, &c.Email, &c.DeletedAt, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (s *Store) ListClients(ctx context.Context) ([]Client, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT client_uuid, client_id, name, email, deleted_at, created_at, updated_at
		FROM clients 
		WHERE deleted_at IS NULL
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []Client
	for rows.Next() {
		var c Client
		err := rows.Scan(&c.ClientUUID, &c.ClientID, &c.Name, &c.Email, &c.DeletedAt, &c.CreatedAt, &c.UpdatedAt)
		if err != nil {
			return nil, err
		}
		clients = append(clients, c)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return clients, nil
}

func (s *Store) ListClientsByUID(ctx context.Context, uid string) ([]Client, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT DISTINCT c.client_uuid, c.client_id, c.name, c.email, c.deleted_at, c.created_at, c.updated_at
		FROM clients c
		JOIN ad_accounts aa ON aa.client_uuid = c.client_uuid
		JOIN user_bm_access uba ON uba.bm_uuid = aa.bm_uuid
		WHERE c.deleted_at IS NULL
		  AND aa.deleted_at IS NULL
		  AND uba.uid = $1
		  AND uba.is_active = TRUE
		ORDER BY c.name
	`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []Client
	for rows.Next() {
		var c Client
		err := rows.Scan(&c.ClientUUID, &c.ClientID, &c.Name, &c.Email, &c.DeletedAt, &c.CreatedAt, &c.UpdatedAt)
		if err != nil {
			return nil, err
		}
		clients = append(clients, c)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return clients, nil
}

type Creative struct {
	CreativeID  string          `json:"creative_id"`
	ClientUUID  string          `json:"client_uuid"`
	AdAccountID string          `json:"ad_account_id"`
	Name        string          `json:"name"`
	Type        string          `json:"type"`
	URL         string          `json:"url"`
	ThumbURL    *string         `json:"thumb_url,omitempty"`
	Link        *string         `json:"link,omitempty"`
	Message     *string         `json:"message,omitempty"`
	MetaData    json.RawMessage `json:"meta_data,omitempty"`
	DeletedAt   *time.Time      `json:"deleted_at,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func (s *Store) CreateCreative(ctx context.Context, c Creative) error {
	_, err := s.DB.Exec(ctx, `
		INSERT INTO creatives(creative_id, client_uuid, ad_account_id, name, type, url, thumb_url, link, message, meta_data)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, c.CreativeID, c.ClientUUID, c.AdAccountID, c.Name, c.Type, c.URL, c.ThumbURL, c.Link, c.Message, c.MetaData)
	return err
}

func (s *Store) GetCreative(ctx context.Context, creativeID string) (Creative, error) {
	var c Creative
	err := s.DB.QueryRow(ctx, `
		SELECT creative_id, client_uuid, ad_account_id, name, type, url, thumb_url, link, message, 
			COALESCE(meta_data,'{}'::jsonb) AS meta_data, deleted_at, created_at, updated_at
		FROM creatives 
		WHERE creative_id=$1 AND deleted_at IS NULL
	`, creativeID).Scan(
		&c.CreativeID, &c.ClientUUID, &c.AdAccountID, &c.Name, &c.Type, &c.URL,
		&c.ThumbURL, &c.Link, &c.Message, &c.MetaData, &c.DeletedAt, &c.CreatedAt, &c.UpdatedAt,
	)
	return c, err
}

var allowedType = map[string]struct{}{
	"image": {},
	"video": {},
}

func (s *Store) ListCreatives(ctx context.Context, adAccountID string, typeFilter string) ([]Creative, error) {
	if typeFilter != "" {
		if _, ok := allowedType[typeFilter]; !ok {
			return nil, fmt.Errorf("invalid typeFilter: %q", typeFilter)
		}
	}

	query := `
		SELECT creative_id, client_uuid, ad_account_id, name, type, url, thumb_url, link, message, 
			COALESCE(meta_data,'{}'::jsonb) AS meta_data, deleted_at, created_at, updated_at
		FROM creatives WHERE deleted_at IS NULL
	`
	args := []any{}
	argsPos := 1

	if adAccountID != "" {
		query += fmt.Sprintf(" AND ad_account_id=$%d", argsPos)
		args = append(args, adAccountID)
		argsPos++
	}

	if typeFilter != "" {
		query += fmt.Sprintf(" AND type=$%d", argsPos)
		args = append(args, typeFilter)
		argsPos++
	}

	query += " ORDER BY created_at DESC LIMIT 100"

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var creatives []Creative
	for rows.Next() {
		var c Creative
		var md []byte

		err := rows.Scan(
			&c.CreativeID, &c.ClientUUID, &c.AdAccountID, &c.Name, &c.Type, &c.URL,
			&c.ThumbURL, &c.Link, &c.Message, &md, &c.DeletedAt, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		c.MetaData = append(json.RawMessage(nil), md...)
		creatives = append(creatives, c)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return creatives, nil
}

func (s *Store) SoftDeleteCreative(ctx context.Context, creativeID string) error {
	result, err := s.DB.Exec(ctx, `
		UPDATE creatives 
		SET deleted_at = now(), updated_at = now() 
		WHERE creative_id = $1 AND deleted_at IS NULL
	`, creativeID)
	if err != nil {
		return err
	}

	rows := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("creative not found or already deleted: %s", creativeID)
	}
	return nil
}

func (s *Store) GetAdAccount(ctx context.Context, adAccountID string) (AdAccount, error) {
	var aa AdAccount
	err := s.DB.QueryRow(ctx, `
		SELECT aa.ad_account_id, aa.client_uuid, aa.bm_uuid,
			COALESCE(bm.bm_id, '') AS bm_id,
			COALESCE(bm.is_active, FALSE) AS bm_is_active,
			aa.ad_account_name, aa.page_id, aa.token_ref,
			aa.is_active, aa.deleted_at, aa.created_at, aa.updated_at
		FROM ad_accounts aa
		LEFT JOIN business_managers bm ON bm.bm_uuid = aa.bm_uuid
		WHERE aa.ad_account_id = $1 AND aa.deleted_at IS NULL
	`, adAccountID).Scan(
		&aa.AdAccountID, &aa.ClientUUID, &aa.BMUUID, &aa.BMID, &aa.BMIsActive,
		&aa.AdAccountName, &aa.PageID, &aa.TokenRef, &aa.IsActive, &aa.DeletedAt, &aa.CreatedAt, &aa.UpdatedAt,
	)
	return aa, err
}

func (s *Store) ListAdAccountsByClient(ctx context.Context, clientUUID string) ([]AdAccount, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT aa.ad_account_id, aa.client_uuid, aa.bm_uuid,
			COALESCE(bm.bm_id, '') AS bm_id,
			COALESCE(bm.is_active, FALSE) AS bm_is_active,
			aa.ad_account_name, aa.page_id, aa.token_ref,
			aa.is_active, aa.deleted_at, aa.created_at, aa.updated_at
		FROM ad_accounts aa
		LEFT JOIN business_managers bm ON bm.bm_uuid = aa.bm_uuid
		WHERE aa.client_uuid = $1 AND aa.deleted_at IS NULL
		ORDER BY aa.ad_account_name
	`, clientUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []AdAccount
	for rows.Next() {
		var aa AdAccount
		err := rows.Scan(
			&aa.AdAccountID, &aa.ClientUUID, &aa.BMUUID, &aa.BMID, &aa.BMIsActive,
			&aa.AdAccountName, &aa.PageID, &aa.TokenRef, &aa.IsActive, &aa.DeletedAt, &aa.CreatedAt, &aa.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, aa)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return accounts, nil
}

func (s *Store) ListAdAccountsByClientForUID(ctx context.Context, clientUUID, uid string) ([]AdAccount, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT aa.ad_account_id, aa.client_uuid, aa.bm_uuid,
			COALESCE(bm.bm_id, '') AS bm_id,
			COALESCE(bm.is_active, FALSE) AS bm_is_active,
			aa.ad_account_name, aa.page_id, aa.token_ref,
			aa.is_active, aa.deleted_at, aa.created_at, aa.updated_at
		FROM ad_accounts aa
		JOIN user_bm_access uba ON uba.bm_uuid = aa.bm_uuid
		LEFT JOIN business_managers bm ON bm.bm_uuid = aa.bm_uuid
		WHERE aa.client_uuid = $1
		  AND aa.deleted_at IS NULL
		  AND uba.uid = $2
		  AND uba.is_active = TRUE
		ORDER BY aa.ad_account_name
	`, clientUUID, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []AdAccount
	for rows.Next() {
		var aa AdAccount
		err := rows.Scan(
			&aa.AdAccountID, &aa.ClientUUID, &aa.BMUUID, &aa.BMID, &aa.BMIsActive,
			&aa.AdAccountName, &aa.PageID, &aa.TokenRef, &aa.IsActive, &aa.DeletedAt, &aa.CreatedAt, &aa.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, aa)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return accounts, nil
}

func (s *Store) CreateAdAccount(ctx context.Context, aa AdAccount) error {
	_, err := s.DB.Exec(ctx, `
		INSERT INTO ad_accounts(ad_account_id, client_uuid, bm_uuid, ad_account_name, page_id, token_ref, is_active)
		VALUES($1, $2, $3, $4, $5, $6, $7)
	`, aa.AdAccountID, aa.ClientUUID, aa.BMUUID, aa.AdAccountName, aa.PageID, aa.TokenRef, aa.IsActive)
	return err
}

func (s *Store) SoftDeleteAdAccount(ctx context.Context, adAccountID string) error {
	result, err := s.DB.Exec(ctx, `
		UPDATE ad_accounts 
		SET deleted_at = now(), updated_at = now() 
		WHERE ad_account_id = $1 AND deleted_at IS NULL
	`, adAccountID)
	if err != nil {
		return err
	}

	rows := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("ad account not found or already deleted: %s", adAccountID)
	}
	return nil
}

func (s *Store) EnsureAppUser(ctx context.Context, uid, email string) error {
	_, err := s.DB.Exec(ctx, `
		INSERT INTO app_users(uid, email)
		VALUES($1, $2)
		ON CONFLICT (uid)
		DO UPDATE SET email = EXCLUDED.email, updated_at = now()
	`, uid, email)
	return err
}

func (s *Store) UserCanAccessAdAccount(ctx context.Context, uid, adAccountID string) (bool, error) {
	var ok bool
	err := s.DB.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM user_bm_access uba
			JOIN ad_accounts aa ON aa.bm_uuid = uba.bm_uuid
			WHERE uba.uid = $1
			  AND uba.is_active = TRUE
			  AND aa.ad_account_id = $2
			  AND aa.deleted_at IS NULL
		)
	`, uid, adAccountID).Scan(&ok)
	return ok, err
}

func (s *Store) UserRoleForAdAccount(ctx context.Context, uid, adAccountID string) (string, bool, error) {
	var role string
	err := s.DB.QueryRow(ctx, `
		SELECT uba.role
		FROM user_bm_access uba
		JOIN ad_accounts aa ON aa.bm_uuid = uba.bm_uuid
		WHERE uba.uid = $1
		  AND uba.is_active = TRUE
		  AND aa.ad_account_id = $2
		  AND aa.deleted_at IS NULL
		LIMIT 1
	`, uid, adAccountID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return role, true, nil
}

func (s *Store) UserRoleForBM(ctx context.Context, uid, bmUUID string) (string, bool, error) {
	var role string
	err := s.DB.QueryRow(ctx, `
		SELECT role
		FROM user_bm_access
		WHERE uid = $1
		  AND bm_uuid = $2
		  AND is_active = TRUE
		LIMIT 1
	`, uid, bmUUID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return role, true, nil
}

func (s *Store) UpsertUserBMAccess(ctx context.Context, uid, bmUUID, role string, isActive bool) error {
	_, err := s.DB.Exec(ctx, `
		INSERT INTO user_bm_access(uid, bm_uuid, role, is_active)
		VALUES($1, $2, $3, $4)
		ON CONFLICT (uid, bm_uuid)
		DO UPDATE SET
			role = EXCLUDED.role,
			is_active = EXCLUDED.is_active,
			updated_at = now()
	`, uid, bmUUID, role, isActive)
	return err
}

var allowedEntityStatusType = map[string]struct{}{
	"creative": {},
	"campaign": {},
	"adset":    {},
	"ad":       {},
}

type EntityStatusUpsert struct {
	EntityType  string
	EntityID    string
	AdAccountID string
	Status      string
	RawPayload  json.RawMessage
}

type EntityStatusRecord struct {
	EntityType  string          `json:"entity_type"`
	EntityID    string          `json:"entity_id"`
	AdAccountID string          `json:"ad_account_id"`
	Status      string          `json:"status"`
	RawPayload  json.RawMessage `json:"raw_payload"`
	SyncedAt    time.Time       `json:"synced_at"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func normalizeEntityStatusType(entityType string) string {
	return strings.ToLower(strings.TrimSpace(entityType))
}

func validateEntityStatusType(entityType string) error {
	entityType = normalizeEntityStatusType(entityType)
	if _, ok := allowedEntityStatusType[entityType]; !ok {
		return fmt.Errorf("invalid entity_type: %q", entityType)
	}
	return nil
}

func (s *Store) UpsertEntityStatuses(ctx context.Context, items []EntityStatusUpsert) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, item := range items {
		item.EntityType = normalizeEntityStatusType(item.EntityType)
		if err := validateEntityStatusType(item.EntityType); err != nil {
			return err
		}
		if strings.TrimSpace(item.EntityID) == "" {
			return errors.New("entity_id is required")
		}
		if strings.TrimSpace(item.AdAccountID) == "" {
			return errors.New("ad_account_id is required")
		}

		raw := item.RawPayload
		if len(raw) == 0 {
			raw = json.RawMessage(`{}`)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO entity_status_cache(entity_type, entity_id, ad_account_id, status, raw_payload, synced_at)
			VALUES($1, $2, $3, $4, $5, now())
			ON CONFLICT (entity_type, entity_id)
			DO UPDATE SET
				ad_account_id = EXCLUDED.ad_account_id,
				status = EXCLUDED.status,
				raw_payload = EXCLUDED.raw_payload,
				synced_at = now(),
				updated_at = now()
		`, item.EntityType, item.EntityID, item.AdAccountID, item.Status, raw); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *Store) ListEntityStatuses(
	ctx context.Context,
	adAccountID string,
	entityType string,
) ([]EntityStatusRecord, error) {
	adAccountID = strings.TrimSpace(adAccountID)
	if adAccountID == "" {
		return nil, errors.New("ad_account_id is required")
	}

	entityType = normalizeEntityStatusType(entityType)
	if entityType != "" {
		if err := validateEntityStatusType(entityType); err != nil {
			return nil, err
		}
	}

	query := `
		SELECT entity_type, entity_id, ad_account_id, COALESCE(status, ''), raw_payload, synced_at, created_at, updated_at
		FROM entity_status_cache
		WHERE ad_account_id = $1
	`
	args := []any{adAccountID}

	if entityType != "" {
		query += " AND entity_type = $2"
		args = append(args, entityType)
	}

	query += " ORDER BY entity_type, synced_at DESC"

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []EntityStatusRecord
	for rows.Next() {
		var rec EntityStatusRecord
		var raw []byte

		if err := rows.Scan(
			&rec.EntityType,
			&rec.EntityID,
			&rec.AdAccountID,
			&rec.Status,
			&raw,
			&rec.SyncedAt,
			&rec.CreatedAt,
			&rec.UpdatedAt,
		); err != nil {
			return nil, err
		}

		rec.RawPayload = append(json.RawMessage(nil), raw...)
		out = append(out, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ListEntityStatusMap(
	ctx context.Context,
	adAccountID string,
	entityType string,
) (map[string]EntityStatusRecord, error) {
	rows, err := s.ListEntityStatuses(ctx, adAccountID, entityType)
	if err != nil {
		return nil, err
	}

	out := make(map[string]EntityStatusRecord, len(rows))
	for _, row := range rows {
		if _, exists := out[row.EntityID]; exists {
			continue
		}
		out[row.EntityID] = row
	}

	return out, nil
}
