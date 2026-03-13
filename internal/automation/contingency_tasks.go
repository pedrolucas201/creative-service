package automation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
)

type ContingencyTaskQueueConfig struct {
	ProjectID     string
	Location      string
	QueueID       string
	ExecuteURL    string
	InternalToken string
}

type ContingencyTaskQueue struct {
	client        *cloudtasks.Client
	queuePath     string
	executeURL    string
	internalToken string
}

func NewContingencyTaskQueue(ctx context.Context, cfg ContingencyTaskQueueConfig) (*ContingencyTaskQueue, error) {
	cfg.ProjectID = strings.TrimSpace(cfg.ProjectID)
	cfg.Location = strings.TrimSpace(cfg.Location)
	cfg.QueueID = strings.TrimSpace(cfg.QueueID)
	cfg.ExecuteURL = strings.TrimSpace(cfg.ExecuteURL)
	cfg.InternalToken = strings.TrimSpace(cfg.InternalToken)

	if cfg.ProjectID == "" {
		return nil, errors.New("project_id is required")
	}
	if cfg.Location == "" {
		return nil, errors.New("location is required")
	}
	if cfg.QueueID == "" {
		return nil, errors.New("queue_id is required")
	}
	if cfg.ExecuteURL == "" {
		return nil, errors.New("execute_url is required")
	}
	if cfg.InternalToken == "" {
		return nil, errors.New("internal_token is required")
	}

	client, err := cloudtasks.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	return &ContingencyTaskQueue{
		client:        client,
		queuePath:     fmt.Sprintf("projects/%s/locations/%s/queues/%s", cfg.ProjectID, cfg.Location, cfg.QueueID),
		executeURL:    cfg.ExecuteURL,
		internalToken: cfg.InternalToken,
	}, nil
}

func (q *ContingencyTaskQueue) Close() error {
	if q == nil || q.client == nil {
		return nil
	}
	return q.client.Close()
}

func (q *ContingencyTaskQueue) EnqueueContingencyExecution(
	ctx context.Context,
	incidentUUID string,
	maxAttempts int,
) (string, error) {
	if q == nil || q.client == nil {
		return "", errors.New("cloud tasks client is not configured")
	}

	incidentUUID = strings.TrimSpace(incidentUUID)
	if incidentUUID == "" {
		return "", errors.New("incident_uuid is required")
	}
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	body, err := json.Marshal(map[string]any{
		"incident_uuid": incidentUUID,
		"max_attempts":  maxAttempts,
	})
	if err != nil {
		return "", err
	}

	req := &taskspb.CreateTaskRequest{
		Parent: q.queuePath,
		Task: &taskspb.Task{
			MessageType: &taskspb.Task_HttpRequest{
				HttpRequest: &taskspb.HttpRequest{
					HttpMethod: taskspb.HttpMethod_POST,
					Url:        q.executeURL,
					Headers: map[string]string{
						"Content-Type":        "application/json",
						"X-Contingency-Token": q.internalToken,
					},
					Body: body,
				},
			},
		},
	}

	created, err := q.client.CreateTask(ctx, req)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(created.GetName()), nil
}
