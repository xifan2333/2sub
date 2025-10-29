package bijian

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	apiBaseURL      = "https://member.bilibili.com/x/bcut/rubick-interface"
	apiReqUpload    = apiBaseURL + "/resource/create"
	apiCommitUpload = apiBaseURL + "/resource/create/complete"
	apiCreateTask   = apiBaseURL + "/task"
	apiQueryResult  = apiBaseURL + "/task/result"
)

// fetch executes the complete Bijian ASR transcription workflow
func fetch(ctx context.Context, audioPath string, opts *Options) (map[string]interface{}, error) {
	// Read audio file
	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		return nil, &FetchError{Step: "read_file", Message: "failed to read audio file", Err: err}
	}

	// Step 1: Request upload
	uploadResp, err := requestUpload(ctx, audioData, opts)
	if err != nil {
		return nil, &FetchError{Step: "request_upload", Message: "failed to request upload", Err: err}
	}

	// Step 2: Upload parts
	etags, err := uploadParts(ctx, audioData, uploadResp, opts)
	if err != nil {
		return nil, &FetchError{Step: "upload_parts", Message: "failed to upload parts", Err: err}
	}

	// Step 3: Commit upload
	downloadURL, err := commitUpload(ctx, uploadResp, etags, opts)
	if err != nil {
		return nil, &FetchError{Step: "commit_upload", Message: "failed to commit upload", Err: err}
	}

	// Step 4: Create transcription task
	taskID, err := createTask(ctx, downloadURL, opts)
	if err != nil {
		return nil, &FetchError{Step: "create_task", Message: "failed to create task", Err: err}
	}

	// Step 5: Poll for result
	result, err := pollResult(ctx, taskID, opts)
	if err != nil {
		return nil, &FetchError{Step: "poll_result", Message: "failed to poll result", Err: err}
	}

	return result, nil
}

// requestUpload requests upload authorization
func requestUpload(ctx context.Context, audioData []byte, opts *Options) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"type":             2,
		"name":             "audio.mp3",
		"size":             len(audioData),
		"ResourceFileType": "mp3",
		"model_id":         "8",
	}

	resp, err := doRequest(ctx, "POST", apiReqUpload, payload, opts)
	if err != nil {
		return nil, err
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing data field in response")
	}

	return data, nil
}

// uploadParts uploads audio parts
func uploadParts(ctx context.Context, audioData []byte, uploadResp map[string]interface{}, opts *Options) ([]string, error) {
	uploadURLs, ok := uploadResp["upload_urls"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing upload_urls in response")
	}

	perSize, ok := uploadResp["per_size"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing per_size in response")
	}

	var etags []string
	for i, urlInterface := range uploadURLs {
		url, ok := urlInterface.(string)
		if !ok {
			return nil, fmt.Errorf("invalid upload_url at index %d", i)
		}

		start := i * int(perSize)
		end := (i + 1) * int(perSize)
		if end > len(audioData) {
			end = len(audioData)
		}

		etag, err := uploadPart(ctx, url, audioData[start:end], opts)
		if err != nil {
			return nil, fmt.Errorf("failed to upload part %d: %w", i, err)
		}
		etags = append(etags, etag)
	}

	return etags, nil
}

// uploadPart uploads a single part
func uploadPart(ctx context.Context, url string, data []byte, opts *Options) (string, error) {
	req, err := http.NewRequest("PUT", url, bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Bilibili/1.0.0 (https://www.bilibili.com)")
	req.Header.Set("Content-Type", "application/json")
	if opts.Cookie != "" {
		req.Header.Set("Cookie", opts.Cookie)
	}
	req = req.WithContext(ctx)

	client := &http.Client{Timeout: 2 * time.Hour}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", &APIError{StatusCode: resp.StatusCode, Response: string(body)}
	}

	etag := resp.Header.Get("Etag")
	return etag, nil
}

// commitUpload commits the upload
func commitUpload(ctx context.Context, uploadResp map[string]interface{}, etags []string, opts *Options) (string, error) {
	payload := map[string]interface{}{
		"InBossKey":  uploadResp["in_boss_key"],
		"ResourceId": uploadResp["resource_id"],
		"Etags":      strings.Join(etags, ","),
		"UploadId":   uploadResp["upload_id"],
		"model_id":   "8",
	}

	resp, err := doRequest(ctx, "POST", apiCommitUpload, payload, opts)
	if err != nil {
		return "", err
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("missing data field in response")
	}

	downloadURL, ok := data["download_url"].(string)
	if !ok {
		return "", fmt.Errorf("missing download_url in response")
	}

	return downloadURL, nil
}

// createTask creates a transcription task
func createTask(ctx context.Context, downloadURL string, opts *Options) (string, error) {
	payload := map[string]interface{}{
		"resource": downloadURL,
		"model_id": "8",
	}

	resp, err := doRequest(ctx, "POST", apiCreateTask, payload, opts)
	if err != nil {
		return "", err
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("missing data field in response")
	}

	taskID, ok := data["task_id"].(string)
	if !ok {
		return "", fmt.Errorf("missing task_id in response")
	}

	return taskID, nil
}

// pollResult polls for task result
func pollResult(ctx context.Context, taskID string, opts *Options) (map[string]interface{}, error) {
	for i := 0; i < 500; i++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		resp, err := queryResult(ctx, taskID, opts)
		if err != nil {
			return nil, err
		}

		data, ok := resp["data"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("missing data field in response")
		}

		state, ok := data["state"].(float64)
		if !ok {
			return nil, fmt.Errorf("missing state in response")
		}

		// state == 4 means completed
		if state == 4 {
			resultStr, ok := data["result"].(string)
			if !ok {
				return nil, fmt.Errorf("missing result in response")
			}

			// Parse result JSON string
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
				return nil, fmt.Errorf("failed to parse result JSON: %w", err)
			}

			return result, nil
		}

		time.Sleep(1 * time.Second)
	}

	return nil, fmt.Errorf("polling timeout after 500 attempts")
}

// queryResult queries task result
func queryResult(ctx context.Context, taskID string, opts *Options) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s?model_id=7&task_id=%s", apiQueryResult, taskID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Bilibili/1.0.0 (https://www.bilibili.com)")
	if opts.Cookie != "" {
		req.Header.Set("Cookie", opts.Cookie)
	}
	req = req.WithContext(ctx)

	client := &http.Client{Timeout: 2 * time.Hour}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{StatusCode: resp.StatusCode, Response: string(body)}
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return result, nil
}

// doRequest executes an HTTP JSON request
func doRequest(ctx context.Context, method, url string, payload map[string]interface{}, opts *Options) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Bilibili/1.0.0 (https://www.bilibili.com)")
	req.Header.Set("Content-Type", "application/json")
	if opts.Cookie != "" {
		req.Header.Set("Cookie", opts.Cookie)
	}
	req = req.WithContext(ctx)

	client := &http.Client{Timeout: 2 * time.Hour}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{StatusCode: resp.StatusCode, Response: string(body)}
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return result, nil
}
