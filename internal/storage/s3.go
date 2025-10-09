/*
Copyright (c) 2025 Odd Kin <oddkin@oddkin.co>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// S3Backend implements StorageBackend for S3-compatible storage
type S3Backend struct {
	endpoint   string
	bucket     string
	region     string
	accessKey  string
	secretKey  string
	useSSL     bool
	httpClient *http.Client
}

// S3Config holds configuration for S3-compatible storage
type S3Config struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

// NewS3Backend creates a new S3-compatible storage backend
func NewS3Backend(config S3Config) *S3Backend {
	return &S3Backend{
		endpoint:  config.Endpoint,
		bucket:    config.Bucket,
		region:    config.Region,
		accessKey: config.AccessKey,
		secretKey: config.SecretKey,
		useSSL:    config.UseSSL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Store uploads data to S3-compatible storage
func (s *S3Backend) Store(ctx context.Context, key string, data []byte) (string, error) {
	// Construct the URL
	objectURL := s.buildObjectURL(key)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "PUT", objectURL, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(data)))

	// Add authentication headers (simplified - in production use proper AWS signature)
	if s.accessKey != "" && s.secretKey != "" {
		// This is a simplified auth approach - in production, implement proper AWS Signature V4
		req.Header.Set("Authorization", fmt.Sprintf("AWS %s:%s", s.accessKey, s.secretKey))
	}

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("S3 upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return objectURL, nil
}

// List returns a list of keys with the given prefix
func (s *S3Backend) List(ctx context.Context, prefix string) ([]string, error) {
	// Construct the list URL
	listURL := s.buildListURL(prefix)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", listURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list request: %w", err)
	}

	// Add authentication headers
	if s.accessKey != "" && s.secretKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("AWS %s:%s", s.accessKey, s.secretKey))
	}

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("S3 list failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response (simplified - in production, parse XML response properly)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read list response: %w", err)
	}

	// This is a simplified parser - in production, use proper XML parsing
	keys := s.parseListResponse(string(body), prefix)
	return keys, nil
}

// Delete removes an object from S3-compatible storage
func (s *S3Backend) Delete(ctx context.Context, key string) error {
	// Construct the URL
	objectURL := s.buildObjectURL(key)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "DELETE", objectURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	// Add authentication headers
	if s.accessKey != "" && s.secretKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("AWS %s:%s", s.accessKey, s.secretKey))
	}

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	defer resp.Body.Close()

	// Check response status (204 No Content is success for DELETE)
	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("S3 delete failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetURL returns the URL for accessing the stored object
func (s *S3Backend) GetURL(key string) string {
	return s.buildObjectURL(key)
}

// buildObjectURL constructs the URL for an object
func (s *S3Backend) buildObjectURL(key string) string {
	scheme := "https"
	if !s.useSSL {
		scheme = "http"
	}

	// Clean the key to ensure proper path
	cleanKey := strings.TrimPrefix(key, "/")
	
	return fmt.Sprintf("%s://%s/%s/%s", scheme, s.endpoint, s.bucket, cleanKey)
}

// buildListURL constructs the URL for listing objects
func (s *S3Backend) buildListURL(prefix string) string {
	scheme := "https"
	if !s.useSSL {
		scheme = "http"
	}

	baseURL := fmt.Sprintf("%s://%s/%s", scheme, s.endpoint, s.bucket)
	
	// Add query parameters for listing
	params := url.Values{}
	if prefix != "" {
		params.Set("prefix", prefix)
	}
	params.Set("list-type", "2") // Use ListObjectsV2

	if len(params) > 0 {
		return baseURL + "?" + params.Encode()
	}
	return baseURL
}

// parseListResponse parses the S3 list response (simplified implementation)
func (s *S3Backend) parseListResponse(body, prefix string) []string {
	var keys []string
	
	// This is a very simplified parser - in production, use proper XML parsing
	// Look for <Key> tags in the XML response
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "<Key>") && strings.HasSuffix(line, "</Key>") {
			key := strings.TrimPrefix(line, "<Key>")
			key = strings.TrimSuffix(key, "</Key>")
			if strings.HasPrefix(key, prefix) {
				keys = append(keys, key)
			}
		}
	}
	
	return keys
}