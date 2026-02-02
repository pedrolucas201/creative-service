package storage

import (
	"context"
	"encoding/json"
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
