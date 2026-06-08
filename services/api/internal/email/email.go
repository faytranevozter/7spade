// Package email abstracts transactional email sending for the API (password
// reset and email verification links).
//
// Two implementations are provided:
//   - LogSender writes the link to the application log. It is the default in
//     development so no SMTP server is required; the URL can be copied from the
//     console to complete a flow.
//   - SMTPSender delivers real mail via net/smtp, configured from SMTP_* env.
//
// NewFromConfig picks SMTPSender when SMTP_HOST is set, otherwise LogSender.
package email

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"net/smtp"
	"strings"
)

// Sender delivers transactional emails. Implementations must be safe for
// concurrent use.
type Sender interface {
	SendPasswordReset(ctx context.Context, to, link string) error
	SendVerification(ctx context.Context, to, link string) error
}

var (
	passwordResetTmpl = template.Must(template.New("reset").Parse(`<!doctype html>
<html><body style="font-family:sans-serif;line-height:1.5">
<h2>Reset your Seven Spade password</h2>
<p>We received a request to reset your password. This link expires in 15 minutes and can be used once.</p>
<p><a href="{{.Link}}">Reset your password</a></p>
<p>If you didn't request this, you can safely ignore this email.</p>
</body></html>`))

	verificationTmpl = template.Must(template.New("verify").Parse(`<!doctype html>
<html><body style="font-family:sans-serif;line-height:1.5">
<h2>Verify your Seven Spade email</h2>
<p>Confirm your email address to secure your account. This link expires in 24 hours.</p>
<p><a href="{{.Link}}">Verify your email</a></p>
<p>If you didn't create an account, you can safely ignore this email.</p>
</body></html>`))
)

type linkData struct{ Link string }

func renderHTML(tmpl *template.Template, link string) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, linkData{Link: link}); err != nil {
		return "", fmt.Errorf("email: render template: %w", err)
	}
	return buf.String(), nil
}

// LogSender writes the link to the log instead of sending mail. Used in dev.
type LogSender struct{}

func (LogSender) SendPasswordReset(_ context.Context, to, link string) error {
	log.Printf("email(dev): password reset for %s: %s", to, link)
	return nil
}

func (LogSender) SendVerification(_ context.Context, to, link string) error {
	log.Printf("email(dev): verify email for %s: %s", to, link)
	return nil
}

// SMTPSender sends mail through an SMTP server.
type SMTPSender struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

func (s SMTPSender) send(to, subject, htmlBody string) error {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	var auth smtp.Auth
	if s.User != "" {
		auth = smtp.PlainAuth("", s.User, s.Pass, s.Host)
	}
	var msg bytes.Buffer
	fmt.Fprintf(&msg, "From: %s\r\n", s.From)
	fmt.Fprintf(&msg, "To: %s\r\n", to)
	fmt.Fprintf(&msg, "Subject: %s\r\n", subject)
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(htmlBody)
	if err := smtp.SendMail(addr, auth, s.From, []string{to}, msg.Bytes()); err != nil {
		return fmt.Errorf("email: send to %s: %w", to, err)
	}
	return nil
}

func (s SMTPSender) SendPasswordReset(_ context.Context, to, link string) error {
	body, err := renderHTML(passwordResetTmpl, link)
	if err != nil {
		return err
	}
	return s.send(to, "Reset your Seven Spade password", body)
}

func (s SMTPSender) SendVerification(_ context.Context, to, link string) error {
	body, err := renderHTML(verificationTmpl, link)
	if err != nil {
		return err
	}
	return s.send(to, "Verify your Seven Spade email", body)
}

// Config is the subset of app config the email package needs.
type Config struct {
	SMTPHost string
	SMTPPort int
	SMTPUser string
	SMTPPass string
	SMTPFrom string
}

// NewFromConfig returns an SMTPSender when SMTP_HOST is configured, otherwise a
// LogSender (dev default — no SMTP server required).
func NewFromConfig(cfg Config) Sender {
	if strings.TrimSpace(cfg.SMTPHost) == "" {
		log.Printf("email: SMTP_HOST not set, using dev log sender (links printed to console)")
		return LogSender{}
	}
	return SMTPSender{
		Host: cfg.SMTPHost,
		Port: cfg.SMTPPort,
		User: cfg.SMTPUser,
		Pass: cfg.SMTPPass,
		From: cfg.SMTPFrom,
	}
}
