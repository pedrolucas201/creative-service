package bm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"creative-service/internal/secrets"
)

type Service struct {
	DB *pgxpool.Pool
	SM *secrets.SMResolver
}

type bmRow struct {
	ProjectID  string
	SecretName string
	IsActive   bool
}

func (s *Service) GetBMConfig(ctx context.Context, bmUUID string) (Config, error) {
	if bmUUID == "" {
		return Config{}, errors.New("bm_uuid vazio")
	}

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var row bmRow
	err := s.DB.QueryRow(cctx, `
		SELECT project_id, secret_name, is_active
		FROM business_managers
		WHERE bm_uuid = $1
	`, bmUUID).Scan(&row.ProjectID, &row.SecretName, &row.IsActive)
	if err != nil {
		return Config{}, err
	}
	if !row.IsActive {
		return Config{}, errors.New("bm inativa")
	}
	if row.ProjectID == "" || row.SecretName == "" {
		return Config{}, errors.New("bm sem project_id ou secret_name")
	}

	payload, err := s.SM.Resolve("SM:" + row.ProjectID + "/" + row.SecretName)
	if err != nil {
		return Config{}, err
	}

	payload = strings.TrimPrefix(payload, "\uFEFF")

	var cfg Config
	if err := json.Unmarshal([]byte(payload), &cfg); err != nil {
		return Config{}, err
	}
	if cfg.TokenRef == "" {
		return Config{}, errors.New("token_ref ausente no JSON do secret")
	}

	return cfg, nil
}

func (s *Service) GetBMConfigByAdAccountID(ctx context.Context, adAccountID string) (Config, error) {
	if adAccountID == "" {
		return Config{}, errors.New("ad_account_id vazio")
	}

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var bmUUID string
	err := s.DB.QueryRow(cctx, `
		SELECT bm_uuid
		FROM ad_accounts
		WHERE ad_account_id = $1
		  AND deleted_at IS NULL
		  AND bm_uuid IS NOT NULL
	`, adAccountID).Scan(&bmUUID)
	if err != nil {
		return Config{}, fmt.Errorf("bm_uuid nao encontrado para ad_account_id %s: %w", adAccountID, err)
	}

	cfg, err := s.GetBMConfig(cctx, bmUUID)
	if err != nil {
		return Config{}, err
	}

	if cfg.AdAccountID != "" && cfg.AdAccountID != adAccountID {
		return Config{}, fmt.Errorf("ad_account_id inconsistente: request=%s secret=%s", adAccountID, cfg.AdAccountID)
	}

	return cfg, nil
}
