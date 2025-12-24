package face

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client interface {
	VerifyFace(ctx context.Context, imageBase64 string, userID int64, storedEmbeddings []string) (matched bool, score float64, err error)
	Enroll(ctx context.Context, userID int64, images []string) ([]EnrollEmbedding, error)
}

type httpClient struct {
	baseURL string
	bypass  bool
	client  *http.Client
}

type verifyRequest struct {
	UserID           string   `json:"user_id"`
	ImageBase64      string   `json:"image"`
	StoredEmbeddings []string `json:"stored_embeddings"`
}

type verifyResponse struct {
	Matched bool    `json:"matched"`
	Score   float64 `json:"score"`
}

type enrollRequest struct {
	UserID string   `json:"user_id"`
	Images []string `json:"images"`
}

type EnrollEmbedding struct {
	Vector string `json:"vector"`
	Model  string `json:"model"`
}

type enrollResponse struct {
	Success    bool              `json:"success"`
	Embeddings []EnrollEmbedding `json:"embeddings"`
}

func New(baseURL string, bypass bool) Client {
	return &httpClient{
		baseURL: baseURL,
		bypass:  bypass,
		client: &http.Client{
			// Loading face models can be slow on first request; use generous timeout.
			Timeout: 60 * time.Second,
		},
	}
}

func (c *httpClient) VerifyFace(ctx context.Context, imageBase64 string, userID int64, storedEmbeddings []string) (bool, float64, error) {
	if c.bypass {
		return true, 1.0, nil
	}

	reqBody := verifyRequest{
		UserID:           fmt.Sprintf("%d", userID),
		ImageBase64:      imageBase64,
		StoredEmbeddings: storedEmbeddings,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return false, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/verify", bytes.NewReader(bodyBytes))
	if err != nil {
		return false, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, 0, fmt.Errorf("face service error: status %d", resp.StatusCode)
	}

	var parsed verifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return false, 0, err
	}

	s, _ := json.MarshalIndent(parsed, "", "\t")
	fmt.Print(string(s))

	return parsed.Matched, parsed.Score, nil
}

func (c *httpClient) Enroll(ctx context.Context, userID int64, images []string) ([]EnrollEmbedding, error) {
	if len(images) == 0 {
		return nil, fmt.Errorf("images is required")
	}
	reqBody := enrollRequest{
		UserID: fmt.Sprintf("%d", userID),
		Images: images,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/enroll", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("face service error: status %d", resp.StatusCode)
	}

	var parsed enrollResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if !parsed.Success {
		return nil, fmt.Errorf("face service enroll failed")
	}
	return parsed.Embeddings, nil
}
