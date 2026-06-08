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
	if _, ok := s.(SMTPSender); !ok {
		t.Fatalf("configured SMTP host should yield an SMTPSender, got %T", s)
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

func TestTemplatesRenderLink(t *testing.T) {
	link := "https://app.test/reset-password?token=abc123"
	body, err := renderHTML(passwordResetTmpl, link)
	if err != nil {
		t.Fatalf("render reset: %v", err)
	}
	if !strings.Contains(body, link) {
		t.Fatalf("reset body missing link: %s", body)
	}
	vbody, err := renderHTML(verificationTmpl, "https://app.test/verify-email?token=xyz")
	if err != nil {
		t.Fatalf("render verify: %v", err)
	}
	if !strings.Contains(vbody, "verify-email?token=xyz") {
		t.Fatalf("verify body missing link: %s", vbody)
	}
}
