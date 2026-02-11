package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ DB *pgxpool.Pool }

func New(db *pgxpool.Pool) *Store { return &Store{DB: db} }

type Client struct {
	ClientUUID  string     `json:"client_uuid"`
	ClientID    string     `json:"client_id"`
	Name        string     `json:"name"`
	Email       *string    `json:"email,omitempty"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type AdAccount struct {
	AdAccountID   string     `json:"ad_account_id"` // PK: act_123456789 (Meta ID)
	ClientUUID    string     `json:"client_uuid"`
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

type Creative struct {
	CreativeID      string          `json:"creative_id"`
	ClientUUID      string          `json:"client_uuid"`
	AdAccountID     string          `json:"ad_account_id"` // FK: act_123456789
	Name            string          `json:"name"`
	Type            string          `json:"type"` // image ou video
	S3URL           string          `json:"s3_url"`
	S3ThumbURL      *string         `json:"s3_thumb_url,omitempty"`
	Link            *string         `json:"link,omitempty"`
	Message         *string         `json:"message,omitempty"`
	MetaData        json.RawMessage `json:"meta_data,omitempty"`
	DeletedAt       *time.Time      `json:"deleted_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

func (s *Store) CreateCreative(ctx context.Context, c Creative) error {
	_, err := s.DB.Exec(ctx, `
		INSERT INTO creatives(creative_id, client_uuid, ad_account_id, name, type, s3_url, s3_thumb_url, link, message, meta_data)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, c.CreativeID, c.ClientUUID, c.AdAccountID, c.Name, c.Type, c.S3URL, c.S3ThumbURL, c.Link, c.Message, c.MetaData)
	return err
}

func (s *Store) GetCreative(ctx context.Context, creativeID string) (Creative, error) {
	var c Creative
	err := s.DB.QueryRow(ctx, `
		SELECT creative_id, client_uuid, ad_account_id, name, type, s3_url, s3_thumb_url, link, message, 
			COALESCE(meta_data,'{}'::jsonb) AS meta_data, deleted_at, created_at, updated_at
		FROM creatives 
		WHERE creative_id=$1 AND deleted_at IS NULL
	`, creativeID).Scan(
		&c.CreativeID, &c.ClientUUID, &c.AdAccountID, &c.Name, &c.Type, &c.S3URL,
		&c.S3ThumbURL, &c.Link, &c.Message, &c.MetaData, &c.DeletedAt, &c.CreatedAt, &c.UpdatedAt,
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
		SELECT creative_id, client_uuid, ad_account_id, name, type, s3_url, s3_thumb_url, link, message, 
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
			&c.CreativeID, &c.ClientUUID, &c.AdAccountID, &c.Name, &c.Type, &c.S3URL,
			&c.S3ThumbURL, &c.Link, &c.Message, &md, &c.DeletedAt, &c.CreatedAt, &c.UpdatedAt,
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

// SoftDeleteCreative marca um creative como deletado (soft delete)
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

// ======= Ad Accounts =======

// GetAdAccount busca uma ad account pelo ad_account_id (PK)
func (s *Store) GetAdAccount(ctx context.Context, adAccountID string) (AdAccount, error) {
	var aa AdAccount
	err := s.DB.QueryRow(ctx, `
		SELECT ad_account_id, client_uuid, ad_account_name, page_id, token_ref, 
			is_active, deleted_at, created_at, updated_at
		FROM ad_accounts 
		WHERE ad_account_id = $1 AND deleted_at IS NULL
	`, adAccountID).Scan(
		&aa.AdAccountID, &aa.ClientUUID, &aa.AdAccountName, &aa.PageID, 
		&aa.TokenRef, &aa.IsActive, &aa.DeletedAt, &aa.CreatedAt, &aa.UpdatedAt,
	)
	return aa, err
}

// ListAdAccountsByClient lista todas as ad accounts de um cliente
func (s *Store) ListAdAccountsByClient(ctx context.Context, clientUUID string) ([]AdAccount, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT ad_account_id, client_uuid, ad_account_name, page_id, token_ref, 
			is_active, deleted_at, created_at, updated_at
		FROM ad_accounts 
		WHERE client_uuid = $1 AND deleted_at IS NULL
		ORDER BY ad_account_name
	`, clientUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []AdAccount
	for rows.Next() {
		var aa AdAccount
		err := rows.Scan(
			&aa.AdAccountID, &aa.ClientUUID, &aa.AdAccountName, &aa.PageID, 
			&aa.TokenRef, &aa.IsActive, &aa.DeletedAt, &aa.CreatedAt, &aa.UpdatedAt,
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

// CreateAdAccount cria uma nova ad account
func (s *Store) CreateAdAccount(ctx context.Context, aa AdAccount) error {
	_, err := s.DB.Exec(ctx, `
		INSERT INTO ad_accounts(ad_account_id, client_uuid, ad_account_name, page_id, token_ref, is_active)
		VALUES($1, $2, $3, $4, $5, $6)
	`, aa.AdAccountID, aa.ClientUUID, aa.AdAccountName, aa.PageID, aa.TokenRef, aa.IsActive)
	return err
}

// SoftDeleteAdAccount marca uma ad account como deletada (soft delete)
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
