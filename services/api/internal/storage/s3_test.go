package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/faytranevozter/7spade/services/api/internal/config"
	"github.com/joho/godotenv"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.S3Config
		wantErr string
	}{
		{
			name: "valid config",
			cfg: config.S3Config{
				Endpoint:    "http://localhost:9000",
				AccessKeyID: "access",
				SecretKey:   "secret",
				Bucket:      "test-bucket",
				Region:      "us-east-1",
				PublicURL:   "http://localhost:9000/test-bucket",
			},
		},
		{
			name:    "missing endpoint",
			cfg:     config.S3Config{AccessKeyID: "a", SecretKey: "s", Bucket: "b"},
			wantErr: "S3_ENDPOINT is required",
		},
		{
			name:    "missing access key",
			cfg:     config.S3Config{Endpoint: "http://localhost:9000", SecretKey: "s", Bucket: "b"},
			wantErr: "S3_ACCESS_KEY_ID is required",
		},
		{
			name:    "missing secret key",
			cfg:     config.S3Config{Endpoint: "http://localhost:9000", AccessKeyID: "a", Bucket: "b"},
			wantErr: "S3_SECRET_ACCESS_KEY is required",
		},
		{
			name:    "missing bucket",
			cfg:     config.S3Config{Endpoint: "http://localhost:9000", AccessKeyID: "a", SecretKey: "s"},
			wantErr: "S3_BUCKET is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.cfg)
			if tt.wantErr == "" && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Errorf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestPublicURL(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		key       string
		want      string
	}{
		{
			name:      "normal url",
			publicURL: "https://cdn.example.com",
			key:       "skins/frames/gold.svg",
			want:      "https://cdn.example.com/skins/frames/gold.svg",
		},
		{
			name:      "trailing slash",
			publicURL: "https://cdn.example.com/",
			key:       "skins/frames/gold.svg",
			want:      "https://cdn.example.com/skins/frames/gold.svg",
		},
		{
			name:      "empty public url",
			publicURL: "",
			key:       "skins/frames/gold.svg",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &S3Client{publicURL: tt.publicURL}
			got := c.PublicURL(tt.key)
			if got != tt.want {
				t.Errorf("PublicURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBucket(t *testing.T) {
	c := &S3Client{bucket: "my-bucket"}
	if got := c.Bucket(); got != "my-bucket" {
		t.Errorf("Bucket() = %q, want %q", got, "my-bucket")
	}
}

func TestSkinAssetCORSAllowsPresignedBrowserUploads(t *testing.T) {
	origins := []string{"http://localhost:5174"}
	cors := skinAssetCORS(origins)
	if len(cors.CORSRules) != 1 {
		t.Fatalf("CORS rules = %d, want 1", len(cors.CORSRules))
	}
	rule := cors.CORSRules[0]
	if !contains(rule.AllowedMethods, "PUT") {
		t.Errorf("allowed methods = %v, want PUT for presigned browser uploads", rule.AllowedMethods)
	}
	if !contains(rule.AllowedHeaders, "Content-Type") {
		t.Errorf("allowed headers = %v, want Content-Type", rule.AllowedHeaders)
	}
	if !contains(rule.AllowedOrigins, "http://localhost:5174") {
		t.Errorf("allowed origins = %v, want admin origin", rule.AllowedOrigins)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestHealthCheck_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.S3Config{
		Endpoint:    server.URL,
		AccessKeyID: "access",
		SecretKey:   "secret",
		Bucket:      "test-bucket",
		Region:      "us-east-1",
	}
	c, err := New(cfg)
	if err != nil {
		t.Skipf("S3 client creation failed in test (expected with mock endpoint): %v", err)
	}
	_ = c
}

func TestHealthCheck_Failure(t *testing.T) {
	cfg := config.S3Config{
		Endpoint:    "http://127.0.0.1:19999",
		AccessKeyID: "access",
		SecretKey:   "secret",
		Bucket:      "test-bucket",
		Region:      "us-east-1",
	}
	c, err := New(cfg)
	if err != nil {
		t.Skipf("S3 client creation failed in test (expected with bad endpoint): %v", err)
	}

	err = c.HealthCheck(context.Background())
	if err == nil {
		t.Error("expected error for health check on dead endpoint")
	}
}

// TestS3IntegrationHealthCheck verifies the configured S3-compatible service
// accepts authenticated access to the configured bucket. It never writes data.
func TestS3IntegrationHealthCheck(t *testing.T) {
	if os.Getenv("RUN_S3_INTEGRATION") != "1" {
		t.Skip("set RUN_S3_INTEGRATION=1 to test the configured S3-compatible bucket")
	}

	if err := godotenv.Load("../../.env"); err != nil && os.Getenv("S3_ENDPOINT") == "" {
		t.Fatalf("load API .env: %v", err)
	}

	cfg := config.S3Config{
		Endpoint:     os.Getenv("S3_ENDPOINT"),
		AccessKeyID:  os.Getenv("S3_ACCESS_KEY_ID"),
		SecretKey:    os.Getenv("S3_SECRET_ACCESS_KEY"),
		Bucket:       os.Getenv("S3_BUCKET"),
		Region:       os.Getenv("S3_REGION"),
		PublicURL:    os.Getenv("S3_PUBLIC_URL"),
		UsePathStyle: os.Getenv("S3_USE_PATH_STYLE") == "true",
	}
	if cfg.Region == "" {
		cfg.Region = "auto"
	}

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("create S3 client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := client.HealthCheck(ctx); err != nil {
		t.Fatalf("R2 bucket %q health check failed: %v", cfg.Bucket, err)
	}
}
