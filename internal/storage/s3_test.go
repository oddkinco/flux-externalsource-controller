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
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewS3Backend(t *testing.T) {
	config := S3Config{
		Endpoint:  "s3.amazonaws.com",
		Bucket:    "my-bucket",
		Region:    "us-east-1",
		AccessKey: "access-key",
		SecretKey: "secret-key",
		UseSSL:    true,
	}

	backend := NewS3Backend(config)

	assert.NotNil(t, backend)
	assert.Equal(t, "s3.amazonaws.com", backend.endpoint)
	assert.Equal(t, "my-bucket", backend.bucket)
	assert.Equal(t, "us-east-1", backend.region)
	assert.Equal(t, "access-key", backend.accessKey)
	assert.Equal(t, "secret-key", backend.secretKey)
	assert.True(t, backend.useSSL)
	assert.NotNil(t, backend.httpClient)
}

func TestS3Backend_buildObjectURL(t *testing.T) {
	tests := []struct {
		name     string
		useSSL   bool
		endpoint string
		bucket   string
		key      string
		expected string
	}{
		{
			name:     "https url",
			useSSL:   true,
			endpoint: "s3.amazonaws.com",
			bucket:   "my-bucket",
			key:      "namespace/source/artifact.tar.gz",
			expected: "https://s3.amazonaws.com/my-bucket/namespace/source/artifact.tar.gz",
		},
		{
			name:     "http url",
			useSSL:   false,
			endpoint: "minio.local",
			bucket:   "artifacts",
			key:      "test/file.tar.gz",
			expected: "http://minio.local/artifacts/test/file.tar.gz",
		},
		{
			name:     "key with leading slash",
			useSSL:   true,
			endpoint: "s3.amazonaws.com",
			bucket:   "my-bucket",
			key:      "/namespace/source/artifact.tar.gz",
			expected: "https://s3.amazonaws.com/my-bucket/namespace/source/artifact.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := &S3Backend{
				endpoint: tt.endpoint,
				bucket:   tt.bucket,
				useSSL:   tt.useSSL,
			}

			url := backend.buildObjectURL(tt.key)
			assert.Equal(t, tt.expected, url)
		})
	}
}

func TestS3Backend_buildListURL(t *testing.T) {
	tests := []struct {
		name     string
		useSSL   bool
		endpoint string
		bucket   string
		prefix   string
		expected string
	}{
		{
			name:     "with prefix",
			useSSL:   true,
			endpoint: "s3.amazonaws.com",
			bucket:   "my-bucket",
			prefix:   "namespace/source",
			expected: "https://s3.amazonaws.com/my-bucket?list-type=2&prefix=namespace%2Fsource",
		},
		{
			name:     "without prefix",
			useSSL:   false,
			endpoint: "minio.local",
			bucket:   "artifacts",
			prefix:   "",
			expected: "http://minio.local/artifacts?list-type=2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := &S3Backend{
				endpoint: tt.endpoint,
				bucket:   tt.bucket,
				useSSL:   tt.useSSL,
			}

			url := backend.buildListURL(tt.prefix)
			assert.Equal(t, tt.expected, url)
		})
	}
}

func TestS3Backend_parseListResponse(t *testing.T) {
	tests := []struct {
		name     string
		response string
		prefix   string
		expected []string
	}{
		{
			name: "multiple keys",
			response: `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult>
    <Name>my-bucket</Name>
    <Prefix>namespace/</Prefix>
    <Contents>
        <Key>namespace/source/artifact1.tar.gz</Key>
        <LastModified>2025-01-01T00:00:00Z</LastModified>
    </Contents>
    <Contents>
        <Key>namespace/source/artifact2.tar.gz</Key>
        <LastModified>2025-01-01T00:00:00Z</LastModified>
    </Contents>
</ListBucketResult>`,
			prefix:   "namespace/",
			expected: []string{"namespace/source/artifact1.tar.gz", "namespace/source/artifact2.tar.gz"},
		},
		{
			name: "no matching keys",
			response: `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult>
    <Name>my-bucket</Name>
    <Contents>
        <Key>other/artifact.tar.gz</Key>
    </Contents>
</ListBucketResult>`,
			prefix:   "namespace/",
			expected: nil,
		},
		{
			name:     "empty response",
			response: `<?xml version="1.0" encoding="UTF-8"?><ListBucketResult></ListBucketResult>`,
			prefix:   "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := &S3Backend{}
			keys := backend.parseListResponse(tt.response, tt.prefix)
			assert.Equal(t, tt.expected, keys)
		})
	}
}

func TestS3Backend_Store(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		key           string
		data          []byte
		includeAuth   bool
		expectedError string
		validateReq   func(t *testing.T, req *http.Request)
	}{
		{
			name:         "successful upload",
			statusCode:   200,
			responseBody: "OK",
			key:          "namespace/source/artifact.tar.gz",
			data:         []byte("test data"),
			includeAuth:  true,
			validateReq: func(t *testing.T, req *http.Request) {
				assert.Equal(t, "PUT", req.Method)
				assert.Equal(t, "application/octet-stream", req.Header.Get("Content-Type"))
				assert.Equal(t, "9", req.Header.Get("Content-Length"))
				assert.Contains(t, req.Header.Get("Authorization"), "AWS")

				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				assert.Equal(t, "test data", string(body))
			},
		},
		{
			name:         "upload without auth",
			statusCode:   200,
			responseBody: "OK",
			key:          "test/file.tar.gz",
			data:         []byte("data"),
			includeAuth:  false,
			validateReq: func(t *testing.T, req *http.Request) {
				assert.Equal(t, "", req.Header.Get("Authorization"))
			},
		},
		{
			name:          "upload failure",
			statusCode:    403,
			responseBody:  "Access Denied",
			key:           "namespace/source/artifact.tar.gz",
			data:          []byte("test data"),
			includeAuth:   true,
			expectedError: "S3 upload failed with status 403",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.validateReq != nil {
					tt.validateReq(t, r)
				}
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			// Create backend pointing to mock server
			config := S3Config{
				Endpoint: strings.TrimPrefix(server.URL, "http://"),
				Bucket:   "test-bucket",
				UseSSL:   false,
			}
			if tt.includeAuth {
				config.AccessKey = "test-key"
				config.SecretKey = "test-secret"
			}
			backend := NewS3Backend(config)

			// Execute test
			ctx := context.Background()
			url, err := backend.Store(ctx, tt.key, tt.data)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.Contains(t, url, tt.key)
			}
		})
	}
}

func TestS3Backend_List(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		prefix        string
		expectedKeys  []string
		expectedError string
	}{
		{
			name:       "successful list",
			statusCode: 200,
			responseBody: `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult>
    <Contents>
        <Key>namespace/source/artifact1.tar.gz</Key>
    </Contents>
    <Contents>
        <Key>namespace/source/artifact2.tar.gz</Key>
    </Contents>
</ListBucketResult>`,
			prefix:       "namespace/",
			expectedKeys: []string{"namespace/source/artifact1.tar.gz", "namespace/source/artifact2.tar.gz"},
		},
		{
			name:          "list failure",
			statusCode:    404,
			responseBody:  "Not Found",
			prefix:        "namespace/",
			expectedError: "S3 list failed with status 404",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			// Create backend
			backend := NewS3Backend(S3Config{
				Endpoint:  strings.TrimPrefix(server.URL, "http://"),
				Bucket:    "test-bucket",
				AccessKey: "test-key",
				SecretKey: "test-secret",
				UseSSL:    false,
			})

			// Execute test
			ctx := context.Background()
			keys, err := backend.List(ctx, tt.prefix)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedKeys, keys)
			}
		})
	}
}

func TestS3Backend_Delete(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		key           string
		expectedError string
	}{
		{
			name:         "successful delete - 204",
			statusCode:   204,
			responseBody: "",
			key:          "namespace/source/artifact.tar.gz",
		},
		{
			name:         "successful delete - 200",
			statusCode:   200,
			responseBody: "OK",
			key:          "namespace/source/artifact.tar.gz",
		},
		{
			name:          "delete failure",
			statusCode:    404,
			responseBody:  "Not Found",
			key:           "namespace/source/artifact.tar.gz",
			expectedError: "S3 delete failed with status 404",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "DELETE", r.Method)
				assert.Contains(t, r.URL.Path, tt.key)
				w.WriteHeader(tt.statusCode)
				if tt.responseBody != "" {
					_, _ = w.Write([]byte(tt.responseBody))
				}
			}))
			defer server.Close()

			// Create backend
			backend := NewS3Backend(S3Config{
				Endpoint:  strings.TrimPrefix(server.URL, "http://"),
				Bucket:    "test-bucket",
				AccessKey: "test-key",
				SecretKey: "test-secret",
				UseSSL:    false,
			})

			// Execute test
			ctx := context.Background()
			err := backend.Delete(ctx, tt.key)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestS3Backend_GetURL(t *testing.T) {
	backend := NewS3Backend(S3Config{
		Endpoint: "s3.amazonaws.com",
		Bucket:   "my-bucket",
		UseSSL:   true,
	})

	key := "namespace/source/artifact.tar.gz"
	url := backend.GetURL(key)

	assert.Equal(t, "https://s3.amazonaws.com/my-bucket/namespace/source/artifact.tar.gz", url)
}

func TestS3Backend_Retrieve(t *testing.T) {
	backend := NewS3Backend(S3Config{
		Endpoint: "s3.amazonaws.com",
		Bucket:   "my-bucket",
	})

	ctx := context.Background()
	data, err := backend.Retrieve(ctx, "any-key")

	assert.Error(t, err)
	assert.Nil(t, data)
	assert.Contains(t, err.Error(), "S3 artifacts are accessed directly from S3")
}

func TestS3Backend_Store_ContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Server will be slow, giving time for context cancellation
		<-r.Context().Done()
	}))
	defer server.Close()

	backend := NewS3Backend(S3Config{
		Endpoint: strings.TrimPrefix(server.URL, "http://"),
		Bucket:   "test-bucket",
		UseSSL:   false,
	})

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := backend.Store(ctx, "test/file.tar.gz", []byte("data"))
	require.Error(t, err)
}

func TestS3Backend_HTTPClientError(t *testing.T) {
	// Create backend with invalid endpoint to force HTTP client error
	backend := NewS3Backend(S3Config{
		Endpoint: "invalid-endpoint-that-does-not-exist-12345678:99999",
		Bucket:   "test-bucket",
		UseSSL:   false,
	})

	ctx := context.Background()

	t.Run("Store fails with invalid endpoint", func(t *testing.T) {
		_, err := backend.Store(ctx, "test/file.tar.gz", []byte("data"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to upload to S3")
	})

	t.Run("List fails with invalid endpoint", func(t *testing.T) {
		_, err := backend.List(ctx, "test/")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to list objects")
	})

	t.Run("Delete fails with invalid endpoint", func(t *testing.T) {
		err := backend.Delete(ctx, "test/file.tar.gz")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete object")
	})
}

func TestS3Backend_ParseListResponse_EdgeCases(t *testing.T) {
	backend := &S3Backend{}

	t.Run("malformed XML", func(t *testing.T) {
		response := "<Key>incomplete"
		keys := backend.parseListResponse(response, "")
		assert.Empty(t, keys)
	})

	t.Run("keys without proper tags", func(t *testing.T) {
		response := "Key>namespace/artifact.tar.gz</Key\n<Key namespace/artifact2.tar.gz /Key>"
		keys := backend.parseListResponse(response, "namespace/")
		assert.Empty(t, keys)
	})

	t.Run("empty lines", func(t *testing.T) {
		response := "\n\n<Key>namespace/artifact.tar.gz</Key>\n\n"
		keys := backend.parseListResponse(response, "namespace/")
		assert.Equal(t, []string{"namespace/artifact.tar.gz"}, keys)
	})
}
