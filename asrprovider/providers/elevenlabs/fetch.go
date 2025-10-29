package elevenlabs

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/brianvoe/gofakeit/v6"
)

const (
	apiURL  = "https://api.elevenlabs.io/v1/speech-to-text"
	modelID = "scribe_v1"
)

// fetch executes the ElevenLabs ASR transcription
func fetch(ctx context.Context, audioPath string, opts *Options) (map[string]interface{}, error) {
	// Check if file exists
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		return nil, &FetchError{Step: "check_file", Message: "audio file not found", Err: err}
	}

	// Open file
	file, err := os.Open(audioPath)
	if err != nil {
		return nil, &FetchError{Step: "open_file", Message: "failed to open audio file", Err: err}
	}
	defer file.Close()

	// Create multipart form
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Add file field
	fileName := filepath.Base(audioPath)
	fileWriter, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, &FetchError{Step: "create_form", Message: "failed to create form file", Err: err}
	}

	if _, err := io.Copy(fileWriter, file); err != nil {
		return nil, &FetchError{Step: "copy_file", Message: "failed to copy file content", Err: err}
	}

	// Add other fields
	if err := writer.WriteField("model_id", modelID); err != nil {
		return nil, &FetchError{Step: "add_field", Message: "failed to add model_id field", Err: err}
	}

	if err := writer.WriteField("diarize", "true"); err != nil {
		return nil, &FetchError{Step: "add_field", Message: "failed to add diarize field", Err: err}
	}

	tagAudioEventsStr := "false"
	if opts.TagAudioEvents {
		tagAudioEventsStr = "true"
	}
	if err := writer.WriteField("tag_audio_events", tagAudioEventsStr); err != nil {
		return nil, &FetchError{Step: "add_field", Message: "failed to add tag_audio_events field", Err: err}
	}

	// Add language code if specified and not auto-detection
	if opts.LanguageCode != "" && opts.LanguageCode != "auto" {
		if err := writer.WriteField("language_code", opts.LanguageCode); err != nil {
			return nil, &FetchError{Step: "add_field", Message: "failed to add language_code field", Err: err}
		}
	}

	// Close writer
	if err := writer.Close(); err != nil {
		return nil, &FetchError{Step: "close_writer", Message: "failed to close multipart writer", Err: err}
	}

	// Create request
	req, err := http.NewRequest("POST", apiURL, &requestBody)
	if err != nil {
		return nil, &FetchError{Step: "create_request", Message: "failed to create HTTP request", Err: err}
	}

	// Set headers
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-encoding", "gzip, deflate, br, zstd")
	req.Header.Set("origin", "https://elevenlabs.io")
	req.Header.Set("referer", "https://elevenlabs.io/")
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "same-site")

	// Use gofakeit to generate random browser information
	req.Header.Set("user-agent", gofakeit.UserAgent())
	req.Header.Set("accept-language", generateAcceptLanguage())

	// Add URL parameters
	q := req.URL.Query()
	q.Add("allow_unauthenticated", "1")
	req.URL.RawQuery = q.Encode()

	req = req.WithContext(ctx)

	// Send request
	client := &http.Client{Timeout: 2 * time.Hour}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &FetchError{Step: "http_request", Message: "HTTP request failed", Err: err}
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, &APIError{StatusCode: resp.StatusCode, Response: fmt.Sprintf("failed to read body: %v", err)}
		}
		return nil, &APIError{StatusCode: resp.StatusCode, Response: string(body)}
	}

	// Handle possible gzip compression
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, &FetchError{Step: "decompress", Message: "failed to decompress gzip response", Err: err}
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(reader).Decode(&result); err != nil {
		return nil, &FetchError{Step: "parse_response", Message: "failed to parse JSON response", Err: err}
	}

	return result, nil
}

// generateAcceptLanguage generates a random Accept-Language header
func generateAcceptLanguage() string {
	// Use gofakeit to generate random languages
	languages := []string{
		gofakeit.LanguageAbbreviation(),
		gofakeit.LanguageAbbreviation(),
	}

	// Build Accept-Language string with weights
	if len(languages) >= 2 {
		return fmt.Sprintf("%s,%s;q=0.9,en;q=0.8", languages[0], languages[1])
	}
	return fmt.Sprintf("%s,en;q=0.9", languages[0])
}
