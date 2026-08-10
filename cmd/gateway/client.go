package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// JobClient talks to the job service over HTTP. All job data (queue state,
// storage, counters, activity) lives behind the job service now; the gateway
// holds no direct dependency on Redis or S3.
type JobClient struct {
	baseURL    string
	httpClient *http.Client
	// sseClient has no timeout since SSE connections are long-lived; the
	// regular httpClient's 10s timeout would otherwise kill the stream.
	sseClient *http.Client
}

func NewJobClient(baseURL string) *JobClient {
	return &JobClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		sseClient: &http.Client{},
	}
}

type SubmitRequest struct {
	List  []string `json:"list"`
	Order string   `json:"order"`
}

type IDResponse struct {
	ID string `json:"id"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type CountResponse struct {
	Total     int64 `json:"total"`
	Sorted    int64 `json:"sorted"`
	NotSorted int64 `json:"not_sorted"`
}

type ActivityEntry struct {
	At     time.Time `json:"at"`
	Sorted bool      `json:"sorted"`
	Order  string    `json:"order"`
	List   []string  `json:"list"`
}

type ActivityResponse struct {
	Entries []ActivityEntry `json:"entries"`
}

type UploadResponse struct {
	ID        string `json:"id"`
	UploadURL string `json:"upload_url"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Redis   string `json:"redis"`
	Storage string `json:"storage"`
}

func (c *JobClient) SubmitJob(ctx context.Context, req SubmitRequest) (int, []byte, error) {
	return c.postJSON(ctx, "/jobs", req)
}

func (c *JobClient) GetStatus(ctx context.Context, id string) (int, []byte, error) {
	return c.get(ctx, "/jobs/"+id)
}

func (c *JobClient) SSEStream(ctx context.Context, id string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/jobs/"+id+"/events", nil)
	if err != nil {
		return nil, err
	}
	return c.sseClient.Do(req)
}

func (c *JobClient) CreateUpload(ctx context.Context) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/uploads", nil)
	if err != nil {
		return 0, nil, err
	}
	return c.doAndRead(req)
}

func (c *JobClient) CheckUpload(ctx context.Context, id string, order string) (int, []byte, error) {
	body := map[string]string{"order": order}
	return c.postJSON(ctx, "/uploads/"+id+"/check", body)
}

func (c *JobClient) GetCount(ctx context.Context) (int, []byte, error) {
	return c.get(ctx, "/stats/count")
}

func (c *JobClient) GetActivity(ctx context.Context) (int, []byte, error) {
	return c.get(ctx, "/stats/activity")
}

func (c *JobClient) Health(ctx context.Context) (int, []byte, error) {
	return c.get(ctx, "/health")
}

func (c *JobClient) get(ctx context.Context, path string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return 0, nil, err
	}
	return c.doAndRead(req)
}

func (c *JobClient) postJSON(ctx context.Context, path string, v any) (int, []byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return 0, nil, fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.doAndRead(req)
}

func (c *JobClient) doAndRead(req *http.Request) (int, []byte, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("job service request failed", "method", req.Method, "url", req.URL.String(), "error", err)
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("job service response read failed", "method", req.Method, "url", req.URL.String(), "status", resp.StatusCode, "error", err)
		return resp.StatusCode, nil, err
	}
	if resp.StatusCode >= 500 {
		slog.Warn("job service returned error", "method", req.Method, "url", req.URL.String(), "status", resp.StatusCode, "body", string(body))
	}
	return resp.StatusCode, body, nil
}
