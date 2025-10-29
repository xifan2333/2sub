package jianying

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	apiBaseURL    = "https://lv-pc-api-sinfonlinec.ulikecam.com"
	apiUploadSign = apiBaseURL + "/lv/v1/upload_sign"
	apiSubmit     = apiBaseURL + "/lv/v1/audio_subtitle/submit"
	apiQuery      = apiBaseURL + "/lv/v1/audio_subtitle/query"
	vodBaseURL    = "https://vod.bytedanceapi.com"
	uploadUA      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.138 Safari/537.36 Thea/1.0.1"
)

// uploadContext holds upload-related context information
type uploadContext struct {
	accessKey    string
	secretKey    string
	sessionToken string
	storeURI     string
	auth         string
	uploadID     string
	sessionKey   string
	uploadHost   string
	crc32Hex     string
}

// fetch executes the complete JianYing ASR transcription workflow
func fetch(ctx context.Context, audioPath string, opts *Options) (map[string]interface{}, error) {
	// Read audio file
	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		return nil, &FetchError{Step: "read_file", Message: "failed to read audio file", Err: err}
	}

	// Calculate CRC32
	crc32Val := crc32.ChecksumIEEE(audioData)
	crc32Hex := fmt.Sprintf("%08x", crc32Val)

	// Generate device ID
	tdid := generateTDID()

	uploadCtx := &uploadContext{
		crc32Hex: crc32Hex,
	}

	// Step 1: Get upload signature (AWS credentials)
	if err := getUploadSign(ctx, uploadCtx, tdid); err != nil {
		return nil, &FetchError{Step: "upload_sign", Message: "failed to get upload signature", Err: err}
	}

	// Step 2: Get upload authorization
	if err := getUploadAuth(ctx, uploadCtx, len(audioData)); err != nil {
		return nil, &FetchError{Step: "upload_auth", Message: "failed to get upload authorization", Err: err}
	}

	// Step 3: Upload file
	if err := uploadFile(ctx, uploadCtx, audioData); err != nil {
		return nil, &FetchError{Step: "upload_file", Message: "failed to upload file", Err: err}
	}

	// Step 4: Check upload
	if err := uploadCheck(ctx, uploadCtx); err != nil {
		return nil, &FetchError{Step: "upload_check", Message: "failed to check upload", Err: err}
	}

	// Step 5: Commit upload
	if err := uploadCommit(ctx, uploadCtx, audioData); err != nil {
		return nil, &FetchError{Step: "upload_commit", Message: "failed to commit upload", Err: err}
	}

	// Step 6: Submit transcription task
	queryID, err := submitTask(ctx, uploadCtx, opts, tdid)
	if err != nil {
		return nil, &FetchError{Step: "submit_task", Message: "failed to submit task", Err: err}
	}

	// Step 7: Query result
	result, err := queryTask(ctx, queryID, tdid)
	if err != nil {
		return nil, &FetchError{Step: "query_result", Message: "failed to query result", Err: err}
	}

	return result, nil
}

// generateTDID generates a device ID
func generateTDID() string {
	now := time.Now()
	yearLastDigit := now.Year() % 10
	fr := 390 + yearLastDigit

	var ed string
	if yearLastDigit%2 != 0 {
		ed = "3278516897751"
	} else {
		// Try to use MAC address for uniqueness on even years
		if mac := getTDIDMAC(); mac != "" {
			ed = mac
		} else {
			ed = "1234567890123"
		}
	}

	return fmt.Sprintf("%d%s", fr, ed)
}

// getTDIDMAC returns a formatted MAC address decimal string (13 digits, zero-padded)
func getTDIDMAC() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range ifaces {
		hw := iface.HardwareAddr
		if len(hw) == 0 {
			continue
		}
		var macInt uint64
		for _, b := range hw {
			macInt = (macInt << 8) | uint64(b)
		}
		if macInt == 0 {
			continue
		}
		return fmt.Sprintf("%013d", macInt)
	}

	return ""
}

// getUploadSign gets the upload signature
func getUploadSign(ctx context.Context, uploadCtx *uploadContext, tdid string) error {
	payload := map[string]interface{}{
		"biz": "pc-recognition",
	}

	sign, deviceTime, err := generateSign("/lv/v1/upload_sign", tdid)
	if err != nil {
		return err
	}

	headers := buildHeaders(sign, deviceTime, tdid)
	resp, err := doRequest(ctx, "POST", apiUploadSign, payload, headers)
	if err != nil {
		return err
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing data field in response")
	}

	uploadCtx.accessKey = getStringField(data, "access_key_id")
	uploadCtx.secretKey = getStringField(data, "secret_access_key")
	uploadCtx.sessionToken = getStringField(data, "session_token")

	return nil
}

// getUploadAuth gets upload authorization
func getUploadAuth(ctx context.Context, uploadCtx *uploadContext, fileSize int) error {
	requestParams := fmt.Sprintf("Action=ApplyUploadInner&FileSize=%d&FileType=object&IsInner=1&SpaceName=lv-mac-recognition&Version=2020-11-19&s=5y0udbjapi", fileSize)

	t := time.Now().UTC()
	amzDate := t.Format("20060102T150405Z")
	datestamp := t.Format("20060102")

	headers := map[string]string{
		"x-amz-date":           amzDate,
		"x-amz-security-token": uploadCtx.sessionToken,
	}

	signature := awsSignature(uploadCtx.secretKey, requestParams, headers, "GET", "", "cn", "vod")
	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s/cn/vod/aws4_request, SignedHeaders=x-amz-date;x-amz-security-token, Signature=%s",
		uploadCtx.accessKey, datestamp, signature)

	req, err := http.NewRequest("GET", vodBaseURL+"/?"+requestParams, nil)
	if err != nil {
		return err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("authorization", authHeader)
	req = req.WithContext(ctx)

	client := &http.Client{Timeout: 2 * time.Hour}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return &APIError{StatusCode: resp.StatusCode, Response: string(body)}
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Parse response
	resultData, ok := result["Result"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing Result field in response")
	}

	uploadAddr, ok := resultData["UploadAddress"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing UploadAddress field")
	}

	storeInfos, ok := uploadAddr["StoreInfos"].([]interface{})
	if !ok || len(storeInfos) == 0 {
		return fmt.Errorf("missing or empty StoreInfos")
	}

	storeInfo, ok := storeInfos[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid StoreInfo format")
	}

	uploadHosts, ok := uploadAddr["UploadHosts"].([]interface{})
	if !ok || len(uploadHosts) == 0 {
		return fmt.Errorf("missing or empty UploadHosts")
	}

	uploadCtx.storeURI = getStringField(storeInfo, "StoreUri")
	uploadCtx.auth = getStringField(storeInfo, "Auth")
	uploadCtx.uploadID = getStringField(storeInfo, "UploadID")
	uploadCtx.sessionKey = getStringField(uploadAddr, "SessionKey")
	uploadCtx.uploadHost = getStringField(uploadHosts[0])

	return nil
}

// uploadFile uploads the audio file
func uploadFile(ctx context.Context, uploadCtx *uploadContext, audioData []byte) error {
	reqURL := fmt.Sprintf("https://%s/%s", uploadCtx.uploadHost, uploadCtx.storeURI)

	req, err := http.NewRequest("PUT", reqURL, bytes.NewReader(audioData))
	if err != nil {
		return err
	}

	query := req.URL.Query()
	query.Set("partNumber", "1")
	query.Set("uploadID", uploadCtx.uploadID)
	req.URL.RawQuery = query.Encode()

	req.Header.Set("User-Agent", uploadUA)
	req.Header.Set("Authorization", uploadCtx.auth)
	req.Header.Set("Content-CRC32", uploadCtx.crc32Hex)
	req.Header.Set("Content-Type", "application/octet-stream")
	req = req.WithContext(ctx)

	client := &http.Client{Timeout: 2 * time.Hour}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return &APIError{StatusCode: resp.StatusCode, Response: string(body)}
	}

	if len(body) == 0 {
		return fmt.Errorf("empty response body")
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if success, ok := result["success"].(float64); !ok || success != 0 {
		return fmt.Errorf("unexpected success value: %v", result["success"])
	}

	return nil
}

// uploadCheck checks the upload
func uploadCheck(ctx context.Context, uploadCtx *uploadContext) error {
	reqURL := fmt.Sprintf("https://%s/%s", uploadCtx.uploadHost, uploadCtx.storeURI)
	payload := fmt.Sprintf("1:%s", uploadCtx.crc32Hex)

	req, err := http.NewRequest("POST", reqURL, bytes.NewReader([]byte(payload)))
	if err != nil {
		return err
	}

	query := req.URL.Query()
	query.Set("uploadID", uploadCtx.uploadID)
	req.URL.RawQuery = query.Encode()

	req.Header.Set("User-Agent", uploadUA)
	req.Header.Set("Authorization", uploadCtx.auth)
	req.Header.Set("Content-CRC32", uploadCtx.crc32Hex)
	req = req.WithContext(ctx)

	client := &http.Client{Timeout: 2 * time.Hour}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return &APIError{StatusCode: resp.StatusCode, Response: string(body)}
	}

	if len(body) > 0 {
		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			return fmt.Errorf("failed to parse JSON response: %w", err)
		}

		if success, ok := result["success"].(float64); ok && success != 0 {
			return fmt.Errorf("unexpected success value: %v", result["success"])
		}
	}

	return nil
}

// uploadCommit commits the upload
func uploadCommit(ctx context.Context, uploadCtx *uploadContext, audioData []byte) error {
	reqURL := fmt.Sprintf("https://%s/%s", uploadCtx.uploadHost, uploadCtx.storeURI)

	req, err := http.NewRequest("PUT", reqURL, bytes.NewReader(audioData))
	if err != nil {
		return err
	}

	query := req.URL.Query()
	query.Set("uploadID", uploadCtx.uploadID)
	query.Set("x-amz-security-token", uploadCtx.sessionToken)
	req.URL.RawQuery = query.Encode()

	req.Header.Set("User-Agent", uploadUA)
	req.Header.Set("Authorization", uploadCtx.auth)
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("Content-CRC32", uploadCtx.crc32Hex)
	req = req.WithContext(ctx)

	client := &http.Client{Timeout: 2 * time.Hour}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return &APIError{StatusCode: resp.StatusCode, Response: string(body)}
	}

	if len(body) > 0 {
		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			return fmt.Errorf("failed to parse JSON response: %w", err)
		}

		if success, ok := result["success"].(float64); ok && success != 0 {
			return fmt.Errorf("unexpected success value: %v", result["success"])
		}
	}

	return nil
}

// submitTask submits a transcription task
func submitTask(ctx context.Context, uploadCtx *uploadContext, opts *Options, tdid string) (string, error) {
	payload := map[string]interface{}{
		"adjust_endtime":    200,
		"audio":             uploadCtx.storeURI,
		"caption_type":      2,
		"client_request_id": "45faf98c-160f-4fae-a649-6d89b0fe35be",
		"max_lines":         1,
		"songs_info": []map[string]interface{}{
			{
				"end_time":   opts.EndTime,
				"id":         "",
				"start_time": opts.StartTime,
			},
		},
		"words_per_line": 16,
	}

	sign, deviceTime, err := generateSign("/lv/v1/audio_subtitle/submit", tdid)
	if err != nil {
		return "", err
	}

	headers := buildHeaders(sign, deviceTime, tdid)
	resp, err := doRequest(ctx, "POST", apiSubmit, payload, headers)
	if err != nil {
		return "", err
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("missing data field in response")
	}

	queryID := getStringField(data, "id")
	return queryID, nil
}

// queryTask queries task result
func queryTask(ctx context.Context, queryID string, tdid string) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"id": queryID,
		"pack_options": map[string]interface{}{
			"need_attribute": true,
		},
	}

	sign, deviceTime, err := generateSign("/lv/v1/audio_subtitle/query", tdid)
	if err != nil {
		return nil, err
	}

	headers := buildHeaders(sign, deviceTime, tdid)
	resp, err := doRequest(ctx, "POST", apiQuery, payload, headers)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// generateSign generates a local signature (based on reverse-engineered JavaScript algorithm)
func generateSign(url string, tdid string) (string, string, error) {
	deviceTime := fmt.Sprintf("%d", time.Now().Unix())

	// Extract last 7 characters from URL pathname
	pathname := url
	var v string
	if len(pathname) >= 7 {
		v = pathname[len(pathname)-7:]
	} else {
		v = pathname
	}

	// Build signature string: 9e2c|{v}|{pf}|{appVersion}|{deviceTime}|{tdid}|11ac
	pf := "4"
	appVersion := "6.6.0"
	signString := fmt.Sprintf("9e2c|%s|%s|%s|%s|%s|11ac", v, pf, appVersion, deviceTime, tdid)

	// Calculate MD5 and convert to lowercase
	sign := md5Hash(signString)

	return sign, deviceTime, nil
}

// buildHeaders builds request headers
func buildHeaders(sign, deviceTime, tdid string) map[string]string {
	return map[string]string{
		"User-Agent":  "Cronet/TTNetVersion:d4572e53 2024-06-12 QuicVersion:4bf243e0 2023-04-17",
		"appvr":       "6.6.0",
		"device-time": deviceTime,
		"pf":          "4",
		"sign":        sign,
		"sign-ver":    "1",
		"tdid":        tdid,
	}
}

// doRequest executes an HTTP JSON request
func doRequest(ctx context.Context, method, url string, payload map[string]interface{}, headers map[string]string) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req = req.WithContext(ctx)

	client := &http.Client{Timeout: 2 * time.Hour}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{StatusCode: resp.StatusCode, Response: string(body)}
	}

	var result map[string]interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("failed to parse JSON response: %w", err)
		}
	} else {
		result = make(map[string]interface{})
	}

	// Check ret field
	if ret := getStringField(result, "ret"); ret != "0" {
		errmsg := getStringField(result, "errmsg")
		return nil, fmt.Errorf("API returned error: ret=%s, errmsg=%s", ret, errmsg)
	}

	return result, nil
}

// awsSignature generates AWS signature
func awsSignature(secretKey, requestParams string, headers map[string]string, method, payload, region, service string) string {
	canonicalURI := "/"
	canonicalQuerystring := requestParams

	type headerEntry struct {
		key   string
		value string
	}
	entries := make([]headerEntry, 0, len(headers))
	for k, v := range headers {
		entries = append(entries, headerEntry{
			key:   strings.ToLower(k),
			value: strings.TrimSpace(v),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].key < entries[j].key
	})

	var canonicalHeaderBuilder strings.Builder
	signedHeaderKeys := make([]string, 0, len(entries))
	for _, entry := range entries {
		canonicalHeaderBuilder.WriteString(fmt.Sprintf("%s:%s\n", entry.key, entry.value))
		signedHeaderKeys = append(signedHeaderKeys, entry.key)
	}

	canonicalHeaders := canonicalHeaderBuilder.String()
	signedHeaders := strings.Join(signedHeaderKeys, ";")

	payloadHash := sha256Sum(payload)
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		method, canonicalURI, canonicalQuerystring, canonicalHeaders, signedHeaders, payloadHash)

	amzdate := headers["x-amz-date"]
	datestamp := amzdate[:8]

	algorithm := "AWS4-HMAC-SHA256"
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", datestamp, region, service)
	stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s",
		algorithm, amzdate, credentialScope, sha256Sum(canonicalRequest))

	signingKey := getSignatureKey(secretKey, datestamp, region, service)
	signature := hmacSHA256Hex(signingKey, stringToSign)

	return signature
}

// getSignatureKey generates a signing key
func getSignatureKey(secretKey, dateStamp, regionName, serviceName string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), dateStamp)
	kRegion := hmacSHA256(kDate, regionName)
	kService := hmacSHA256(kRegion, serviceName)
	kSigning := hmacSHA256(kService, "aws4_request")
	return kSigning
}

// hmacSHA256 calculates HMAC-SHA256
func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// hmacSHA256Hex calculates HMAC-SHA256 and returns hex string
func hmacSHA256Hex(key []byte, data string) string {
	return hex.EncodeToString(hmacSHA256(key, data))
}

// sha256Sum calculates SHA256 hash
func sha256Sum(data string) string {
	h := sha256.New()
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// md5Hash calculates MD5 hash and returns lowercase hex string
func md5Hash(data string) string {
	h := md5.New()
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// getStringField safely extracts a string field from interface{}
func getStringField(m interface{}, key ...string) string {
	switch v := m.(type) {
	case map[string]interface{}:
		if len(key) == 0 {
			return ""
		}
		if val, ok := v[key[0]].(string); ok {
			return val
		}
	case string:
		return v
	}
	return ""
}
