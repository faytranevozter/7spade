package storage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPutObjectDoesNotSendOptionalChecksum(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.ContentLength, int64(len("image bytes")); got != want {
			t.Errorf("ContentLength = %d, want %d", got, want)
		}
		if got, want := r.Header.Get("Content-Length"), "11"; got != want {
			t.Errorf("Content-Length = %q, want %q", got, want)
		}
		if got := r.Header.Get("X-Amz-Checksum-Crc32"); got != "" {
			t.Errorf("optional CRC32 header = %q, want empty", got)
		}
		if got := r.Header.Get("X-Amz-Trailer"); got != "" {
			t.Errorf("optional checksum trailer = %q, want empty", got)
		}
		if strings.Contains(r.Header.Get("Content-Encoding"), "aws-chunked") {
			t.Errorf("content encoding = %q, want no aws-chunked checksum stream", r.Header.Get("Content-Encoding"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(body); got != "image bytes" {
			t.Errorf("body = %q, want image bytes", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	signer, err := NewSigner(server.URL, "us-east-1", "skins", "access", "secret", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := signer.PutObject(context.Background(), "skins/test.png", "image/png", int64(len("image bytes")), strings.NewReader("image bytes")); err != nil {
		t.Fatal(err)
	}
}

func TestNewSignerWithPublicURLOnly(t *testing.T) {
	signer, err := NewSigner("", "", "", "", "", "https://cdn.example/assets/", false)
	if err != nil {
		t.Fatal(err)
	}
	if signer == nil {
		t.Fatal("expected public URL signer")
	}
	if got := signer.PublicURL("skins/frame.png"); got != "https://cdn.example/assets/skins/frame.png" {
		t.Fatalf("PublicURL() = %q", got)
	}
	if _, err := signer.PresignPut(context.Background(), "skins/frame.png", "image/png", 10, time.Minute); err == nil || !strings.Contains(err.Error(), "writes are not configured") {
		t.Fatalf("PresignPut() error = %v", err)
	}
	if err := signer.PutObject(context.Background(), "skins/frame.png", "image/png", int64(len("image")), strings.NewReader("image")); err == nil || !strings.Contains(err.Error(), "writes are not configured") {
		t.Fatalf("PutObject() error = %v", err)
	}
	if _, _, err := signer.HeadObject(context.Background(), "skins/frame.png"); err == nil || !strings.Contains(err.Error(), "writes are not configured") {
		t.Fatalf("HeadObject() error = %v", err)
	}
}
