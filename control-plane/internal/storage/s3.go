// Package storage: S3 / MinIO Object Storage Provider adapter.
package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type S3Config struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	PathStyle bool
}

type S3 struct {
	client    *http.Client
	endpoint  *url.URL
	bucket    string
	region    string
	accessKey string
	secretKey string
	pathStyle bool
}

func NewS3(cfg S3Config) (*S3, error) {
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, errors.New("s3 bucket is required")
	}
	endpointStr := strings.TrimSpace(cfg.Endpoint)
	if endpointStr == "" {
		endpointStr = "https://s3.amazonaws.com"
	}
	if !strings.HasPrefix(endpointStr, "http://") && !strings.HasPrefix(endpointStr, "https://") {
		endpointStr = "https://" + endpointStr
	}

	u, err := url.Parse(endpointStr)
	if err != nil {
		return nil, fmt.Errorf("invalid s3 endpoint %q: %w", cfg.Endpoint, err)
	}

	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "us-east-1"
	}

	return &S3{
		client:    &http.Client{Timeout: 30 * time.Second},
		endpoint:  u,
		bucket:    cfg.Bucket,
		region:    region,
		accessKey: strings.TrimSpace(cfg.AccessKey),
		secretKey: strings.TrimSpace(cfg.SecretKey),
		pathStyle: cfg.PathStyle,
	}, nil
}

func (s *S3) Name() string {
	return "s3"
}

func (s *S3) buildURL(key string) string {
	cleanKey := strings.TrimPrefix(key, "/")
	if s.pathStyle {
		base := strings.TrimSuffix(s.endpoint.String(), "/")
		return fmt.Sprintf("%s/%s/%s", base, s.bucket, cleanKey)
	}
	// Virtual-hosted style
	host := s.bucket + "." + s.endpoint.Host
	return fmt.Sprintf("%s://%s/%s", s.endpoint.Scheme, host, cleanKey)
}

func (s *S3) Put(ctx context.Context, key string, content []byte) (Object, error) {
	if strings.TrimSpace(key) == "" {
		return Object{}, errors.New("object key is required")
	}

	targetURL := s.buildURL(key)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, targetURL, bytes.NewReader(content))
	if err != nil {
		return Object{}, fmt.Errorf("create s3 put request: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	s.sign(req, content)

	resp, err := s.client.Do(req)
	if err != nil {
		return Object{}, fmt.Errorf("s3 put %q: %w", key, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return Object{}, fmt.Errorf("s3 put %q returned status %d: %s", key, resp.StatusCode, string(body))
	}

	return Object{
		Key:      key,
		Size:     int64(len(content)),
		Checksum: Checksum(content),
	}, nil
}

func (s *S3) Get(ctx context.Context, key string) ([]byte, error) {
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("object key is required")
	}

	targetURL := s.buildURL(key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create s3 get request: %w", err)
	}

	s.sign(req, nil)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("s3 get %q: %w", key, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, key)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("s3 get %q returned status %d: %s", key, resp.StatusCode, string(body))
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read s3 get body %q: %w", key, err)
	}
	return content, nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("object key is required")
	}

	targetURL := s.buildURL(key)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, targetURL, nil)
	if err != nil {
		return fmt.Errorf("create s3 delete request: %w", err)
	}

	s.sign(req, nil)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("s3 delete %q: %w", key, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("s3 delete %q returned status %d: %s", key, resp.StatusCode, string(body))
	}
	return nil
}

// sign generates AWS Signature Version 4 headers on the HTTP request.
func (s *S3) sign(req *http.Request, body []byte) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	contentHash := sha256Hex(body)
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", contentHash)

	if s.accessKey == "" || s.secretKey == "" {
		return // Unauthenticated request (e.g. public or local mock test)
	}

	host := req.URL.Host
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n", host, contentHash, amzDate)
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	canonicalURI := req.URL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		canonicalURI,
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		contentHash,
	)

	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, s.region)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	)

	signingKey := getSignatureKey(s.secretKey, dateStamp, s.region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.accessKey, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)
}

func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func hmacSHA256(key []byte, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func getSignatureKey(key, dateStamp, regionName, serviceName string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+key), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(regionName))
	kService := hmacSHA256(kRegion, []byte(serviceName))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}
