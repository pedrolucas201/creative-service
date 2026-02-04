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
	ClientID    string
	AdAccountID string
	PageID      string
	TokenRef    string
}

func (s *Store) GetClient(ctx context.Context, clientID string) (Client, error) {
	var c Client
	err := s.DB.QueryRow(ctx,
		`SELECT client_id, ad_account_id, page_id, token_ref FROM clients WHERE client_id=$1`,
		clientID,
	).Scan(&c.ClientID, &c.AdAccountID, &c.PageID, &c.TokenRef)
	return c, err
}

type Job struct {
	JobID         string
	ClientID      string
	JobType       string
	Status        string
	InputJSON     json.RawMessage
	BlobVideoPath *string
	BlobThumbPath *string
	ResultJSON    json.RawMessage
	ErrorText     *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Creative struct {
	CreativeID   string          `json:"creative_id"`
	ClientID     string          `json:"client_id"`
	Name         string          `json:"name"`
	Type         string          `json:"type"` // image ou video
	S3URL        string          `json:"s3_url"`
	S3ThumbURL   *string         `json:"s3_thumb_url,omitempty"`
	Link         *string         `json:"link,omitempty"`
	Message      *string         `json:"message,omitempty"`
	MetaData     json.RawMessage `json:"meta_data,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

func (s *Store) CreateJob(ctx context.Context, j Job) error {
	_, err := s.DB.Exec(ctx, `
		INSERT INTO jobs(job_id, client_id, job_type, status, input_json, blob_video_path, blob_thumb_path)
		VALUES($1,$2,$3,$4,$5,$6,$7)
	`, j.JobID, j.ClientID, j.JobType, j.Status, j.InputJSON, j.BlobVideoPath, j.BlobThumbPath)
	return err
}

func (s *Store) GetJob(ctx context.Context, jobID string) (Job, error) {
	var j Job
	err := s.DB.QueryRow(ctx, `
		SELECT job_id, client_id, job_type, status, input_json, blob_video_path, blob_thumb_path,
		       COALESCE(result_json,'{}'::jsonb), error_text, created_at, updated_at
		FROM jobs WHERE job_id=$1
	`, jobID).Scan(
		&j.JobID, &j.ClientID, &j.JobType, &j.Status, &j.InputJSON,
		&j.BlobVideoPath, &j.BlobThumbPath, &j.ResultJSON, &j.ErrorText, &j.CreatedAt, &j.UpdatedAt,
	)
	return j, err
}

func (s *Store) UpdateJobStatus(ctx context.Context, jobID, status string) error {
	_, err := s.DB.Exec(ctx, `UPDATE jobs SET status=$2, updated_at=now() WHERE job_id=$1`, jobID, status)
	return err
}

func (s *Store) CompleteJob(ctx context.Context, jobID string, result any) error {
	b, _ := json.Marshal(result)
	_, err := s.DB.Exec(ctx, `
		UPDATE jobs SET status='succeeded', result_json=$2, error_text=NULL, updated_at=now()
		WHERE job_id=$1
	`, jobID, b)
	return err
}

func (s *Store) FailJob(ctx context.Context, jobID string, errText string) error {
	_, err := s.DB.Exec(ctx, `
		UPDATE jobs SET status='failed', error_text=$2, updated_at=now()
		WHERE job_id=$1
	`, jobID, errText)
	return err
}

func (s *Store) CreateCreative(ctx context.Context, c Creative) error {
	_, err := s.DB.Exec(ctx, `
		INSERT INTO creatives(creative_id, client_id, name, type, s3_url, s3_thumb_url, link, message, meta_data)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, c.CreativeID, c.ClientID, c.Name, c.Type, c.S3URL, c.S3ThumbURL, c.Link, c.Message, c.MetaData)
	return err
}

func (s *Store) GetCreative(ctx context.Context, creativeID string) (Creative, error) {
	var c Creative
	err := s.DB.QueryRow(ctx, `
		SELECT creative_id, client_id, name, type, s3_url, s3_thumb_url, link, message, 
			COALESCE(meta_data,'{}'::jsonb), created_at, updated_at
		FROM creatives WHERE creative_id=$1
	`, creativeID).Scan(
		&c.CreativeID, &c.ClientID, &c.Name, &c.Type, &c.S3URL,
		&c.S3ThumbURL, &c.Link, &c.Message, &c.MetaData, &c.CreatedAt, &c.UpdatedAt,
	)
	return c, err
}

func (s *Store) ListCreatives(ctx context.Context, clientID string, typeFilter string) ([]Creative, error) {
	query := `
		SELECT creative_id, client_id, name, type, s3_url, s3_thumb_url, link, message, 
			COALESCE(meta_data,'{}'::jsonb), created_at, updated_at
		FROM creatives WHERE 1=1
	`
	args := []any{}
	argsPos := 1

	if clientID != "" {
		query += fmt.Sprintf(" AND client_id=$%d", argsPos)
		args = append(args, clientID)
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
		err := rows.Scan(
			&c.CreativeID, &c.ClientID, &c.Name, &c.Type, &c.S3URL,
			&c.S3ThumbURL, &c.Link, &c.Message, &c.MetaData, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		creatives = append(creatives, c)
	}
	return creatives, nil
}