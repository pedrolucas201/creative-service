package meta

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	APIVersion string
	Token      string
	HTTP       *http.Client
	MaxRetries int
}

func New(baseURL, apiVersion, token string, timeout time.Duration) *Client {
	return &Client{
		BaseURL:    baseURL,
		APIVersion: apiVersion,
		Token:      token,
		HTTP:       &http.Client{Timeout: timeout},
		MaxRetries: 5,
	}
}

func (c *Client) endpoint(path string) string {
	path = strings.TrimPrefix(path, "/")
	return fmt.Sprintf("%s/%s/%s", strings.TrimRight(c.BaseURL, "/"), c.APIVersion, path)
}

type apiError struct {
	FBError struct {
		Message      string `json:"message"`
		Type         string `json:"type"`
		Code         int    `json:"code"`
		ErrorSubcode int    `json:"error_subcode"`
		FbTraceID    string `json:"fbtrace_id"`
	} `json:"error"`
}

func (e apiError) Error() string {
	return fmt.Sprintf(
		"meta api error: code=%d subcode=%d type=%s msg=%s trace=%s",
		e.FBError.Code,
		e.FBError.ErrorSubcode,
		e.FBError.Type,
		e.FBError.Message,
		e.FBError.FbTraceID,
	)
}

func Act(id string) string {
	if strings.HasPrefix(id, "act_") { return id }
	return "act_" + id
}

func (c *Client) doJSON(ctx context.Context, method, path string, q url.Values, body any, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil { return err }
		r = bytes.NewReader(b)
	}
	if q == nil { q = url.Values{} }

	req, err := http.NewRequestWithContext(ctx, method, c.endpoint(path)+"?"+q.Encode(), r)
	if err != nil { return err }
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if body != nil { req.Header.Set("Content-Type", "application/json") }

	respBody, status, err := c.doWithRetry(req)
	if err != nil { return err }
	if status >= 400 {
		var ae apiError
		_ = json.Unmarshal(respBody, &ae)
		if ae.FBError.Code != 0 { return ae }
		return fmt.Errorf("meta http %d: %s", status, string(respBody))
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("unmarshal: %w body=%s", err, string(respBody))
		}
	}
	return nil
}

func (c *Client) doForm(ctx context.Context, method, path string, fields map[string]string, fileField, fileName string, fileBytes []byte, out any) error {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for k, v := range fields { _ = w.WriteField(k, v) }
	if fileField != "" {
		fw, err := w.CreateFormFile(fileField, fileName)
		if err != nil { return err }
		if _, err := fw.Write(fileBytes); err != nil { return err }
	}
	_ = w.Close()

	req, err := http.NewRequestWithContext(ctx, method, c.endpoint(path), &b)
	if err != nil { return err }
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", w.FormDataContentType())

	respBody, status, err := c.doWithRetry(req)
	if err != nil { return err }
	if status >= 400 {
		var ae apiError
		_ = json.Unmarshal(respBody, &ae)
		if ae.FBError.Code != 0 { return ae }
		return fmt.Errorf("meta http %d: %s", status, string(respBody))
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("unmarshal: %w body=%s", err, string(respBody))
		}
	}
	return nil
}

func (c *Client) doWithRetry(req *http.Request) ([]byte, int, error) {
	var lastErr error
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(backoff(attempt))
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode == 429 || (resp.StatusCode >= 500 && resp.StatusCode <= 599) {
			lastErr = fmt.Errorf("retryable status %d: %s", resp.StatusCode, string(body))
			time.Sleep(backoff(attempt))
			continue
		}
		return body, resp.StatusCode, nil
	}
	return nil, 0, fmt.Errorf("failed after retries: %w", lastErr)
}

func backoff(attempt int) time.Duration {
	ms := 250 * (1 << attempt)
	if ms > 8000 { ms = 8000 }
	return time.Duration(ms) * time.Millisecond
}

type UploadVideoResponse struct{ ID string `json:"id"` }

func (c *Client) UploadVideo(ctx context.Context, adAccountID, name, fileName string, mp4 []byte) (string, error) {
	var out UploadVideoResponse
	fields := map[string]string{}
	if name != "" { fields["name"] = name }
	if err := c.doForm(ctx, http.MethodPost, fmt.Sprintf("%s/advideos", Act(adAccountID)), fields, "source", fileName, mp4, &out); err != nil {
		return "", err
	}
	if out.ID == "" { return "", errors.New("upload video: empty id") }
	return out.ID, nil
}

type UploadImageResponse struct {
	Images map[string]struct{ Hash string `json:"hash"` } `json:"images"`
}

func (c *Client) UploadImage(ctx context.Context, adAccountID, fileName string, img []byte) (string, error) {
	var out UploadImageResponse
	if err := c.doForm(ctx, http.MethodPost, fmt.Sprintf("%s/adimages", Act(adAccountID)), nil, "filename", fileName, img, &out); err != nil {
		return "", err
	}
	for _, v := range out.Images {
		if v.Hash != "" { return v.Hash, nil }
	}
	return "", errors.New("upload image: no hash found")
}

type CreateIDResponse struct{ ID string `json:"id"` }

func (c *Client) CreateCreative(ctx context.Context, adAccountID string, payload map[string]any) (string, error) {
	var out CreateIDResponse
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("%s/adcreatives", Act(adAccountID)), nil, payload, &out); err != nil {
		return "", err
	}
	if out.ID == "" { return "", errors.New("create creative: empty id") }
	return out.ID, nil
}

func (c *Client) GetCreative(ctx context.Context, creativeID string, fields []string) (map[string]any, error) {
	q := url.Values{}
	if len(fields) > 0 { q.Set("fields", strings.Join(fields, ",")) }
	var out map[string]any
	if err := c.doJSON(ctx, http.MethodGet, creativeID, q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreateCampaign(ctx context.Context, adAccountID string, payload map[string]any) (string, error) {
	var out CreateIDResponse
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("%s/campaigns", Act(adAccountID)), nil, payload, &out); err != nil {
		return "", err
	}
	if out.ID == "" { return "", errors.New("create campaign: empty id") }
	return out.ID, nil
}

func (c *Client) CreateAdSet(ctx context.Context, adAccountID string, payload map[string]any) (string, error) {
	var out CreateIDResponse
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("%s/adsets", Act(adAccountID)), nil, payload, &out); err != nil {
		return "", err
	}
	if out.ID == "" { return "", errors.New("create adset: empty id") }
	return out.ID, nil
}

func (c *Client) CreateAd(ctx context.Context, adAccountID string, payload map[string]any) (string, error) {
	var out CreateIDResponse
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("%s/ads", Act(adAccountID)), nil, payload, &out); err != nil {
		return "", err
	}
	if out.ID == "" { return "", errors.New("create ad: empty id") }
	return out.ID, nil
}

type ListResponse struct {
	Data   []map[string]any `json:"data"`
	Paging map[string]any   `json:"paging,omitempty"`
}

func (c *Client) ListCampaigns(ctx context.Context, adAccountID string, fields []string) ([]map[string]any, error) {
	q := url.Values{}
	if len(fields) > 0 {
		q.Set("fields", strings.Join(fields, ","))
	}
	var out ListResponse
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("%s/campaigns", Act(adAccountID)), q, nil, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

func (c *Client) ListAdSets(ctx context.Context, adAccountID string, fields []string) ([]map[string]any, error) {
	q := url.Values{}
	if len(fields) > 0 {
		q.Set("fields", strings.Join(fields, ","))
	}
	var out ListResponse
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("%s/adsets", Act(adAccountID)), q, nil, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

func (c *Client) ListAds(ctx context.Context, adAccountID string, fields []string) ([]map[string]any, error) {
	q := url.Values{}
	if len(fields) > 0 {
		q.Set("fields", strings.Join(fields, ","))
	}
	var out ListResponse
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("%s/ads", Act(adAccountID)), q, nil, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}
