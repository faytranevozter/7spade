package email

import (
	"context"
	"strings"
	"testing"
)

func TestNewFromConfigSelectsSender(t *testing.T) {
	if _, ok := NewFromConfig(Config{}).(LogSender); !ok {
		t.Fatal("empty SMTP config should yield a LogSender")
	}
	s := NewFromConfig(Config{SMTPHost: "smtp.example.com", SMTPPort: 587, SMTPFrom: "no-reply@x"})
	sender, ok := s.(SMTPSender)
	if !ok {
		t.Fatalf("configured SMTP host should yield an SMTPSender, got %T", s)
	}
	// AppURL falls back to a sane default when not provided.
	if sender.AppURL == "" {
		t.Fatal("SMTPSender.AppURL should default when AppURL is empty")
	}
}

func TestNewFromConfigCarriesNewFields(t *testing.T) {
	s := NewFromConfig(Config{
		SMTPHost:       "smtp.example.com",
		SMTPPort:       465,
		SMTPFrom:       "no-reply@x",
		SMTPFromName:   "Seven Spade",
		SMTPReplyTo:    "support@x",
		SMTPEncryption: "TLS",
		AppURL:         "https://spade.example.com/",
	}).(SMTPSender)

	if s.FromName != "Seven Spade" {
		t.Fatalf("FromName = %q, want Seven Spade", s.FromName)
	}
	if s.ReplyTo != "support@x" {
		t.Fatalf("ReplyTo = %q, want support@x", s.ReplyTo)
	}
	// Encryption is normalized to lowercase.
	if s.Encryption != EncryptionTLS {
		t.Fatalf("Encryption = %q, want tls", s.Encryption)
	}
	// Trailing slash trimmed.
	if s.AppURL != "https://spade.example.com" {
		t.Fatalf("AppURL = %q, want trimmed", s.AppURL)
	}
}

func TestLogSenderDoesNotError(t *testing.T) {
	ls := LogSender{}
	if err := ls.SendPasswordReset(context.Background(), "a@b.com", "https://x/reset?token=t"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if err := ls.SendVerification(context.Background(), "a@b.com", "https://x/verify?token=t"); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

// render produces HTML + plaintext bodies that both carry the link and the
// per-email copy, wrapped in the shared brand layout.
func TestRenderProducesHTMLAndText(t *testing.T) {
	link := "https://app.test/reset-password?token=abc123"
	html, text, err := render(passwordResetMsg, link, "https://spade.example.com")
	if err != nil {
		t.Fatalf("render reset: %v", err)
	}
	for _, want := range []string{link, passwordResetMsg.ButtonLabel, passwordResetMsg.Heading, "SEVEN SPADE", "https://spade.example.com"} {
		if !strings.Contains(html, want) {
			t.Fatalf("reset HTML missing %q", want)
		}
	}
	for _, want := range []string{link, passwordResetMsg.Heading, "Seven Spade"} {
		if !strings.Contains(text, want) {
			t.Fatalf("reset text missing %q", want)
		}
	}
	// HTML body should not leak raw Go template directives.
	if strings.Contains(html, "{{") {
		t.Fatalf("reset HTML has unrendered template directive: %s", html)
	}

	vhtml, vtext, err := render(verificationMsg, "https://app.test/verify-email?token=xyz", "https://spade.example.com")
	if err != nil {
		t.Fatalf("render verify: %v", err)
	}
	if !strings.Contains(vhtml, "verify-email?token=xyz") || !strings.Contains(vtext, "verify-email?token=xyz") {
		t.Fatalf("verify bodies missing link")
	}
}

// tlsPolicy resolves "auto" by port and honours explicit overrides.
func TestTLSPolicyResolution(t *testing.T) {
	cases := []struct {
		name    string
		enc     Encryption
		port    int
		wantSSL bool
	}{
		{"auto 465 -> implicit TLS", EncryptionAuto, 465, true},
		{"auto 587 -> starttls", EncryptionAuto, 587, false},
		{"empty 465 -> implicit TLS", "", 465, true},
		{"explicit tls", EncryptionTLS, 587, true},
		{"explicit starttls", EncryptionSTARTTLS, 465, false},
		{"explicit none", EncryptionNone, 25, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := SMTPSender{Port: tc.port, Encryption: tc.enc}
			useSSL, _ := s.tlsPolicy()
			if useSSL != tc.wantSSL {
				t.Fatalf("useSSL = %v, want %v", useSSL, tc.wantSSL)
			}
		})
	}
}
