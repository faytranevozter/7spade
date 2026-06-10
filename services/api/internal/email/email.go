// Package email abstracts transactional email sending for the API (password
// reset and email verification links).
//
// Two implementations are provided:
//   - LogSender writes the link to the application log. It is the default in
//     development so no SMTP server is required; the URL can be copied from the
//     console to complete a flow.
//   - SMTPSender delivers real mail via github.com/wneessen/go-mail, configured
//     from SMTP_* env, with a display-name From, HTML + plaintext alternatives,
//     and configurable transport encryption.
//
// NewFromConfig picks SMTPSender when SMTP_HOST is set, otherwise LogSender.
//
// Email markup lives in templates/*.tmpl, embedded at build time. A single
// data-driven layout renders both messages (HTML and plaintext); the per-email
// copy (heading/intro/button/footnote) is supplied by the sender.
package email

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	htmltmpl "html/template"
	"log"
	"strings"
	texttmpl "text/template"
	"time"

	"github.com/wneessen/go-mail"
)

// Sender delivers transactional emails. Implementations must be safe for
// concurrent use.
type Sender interface {
	SendPasswordReset(ctx context.Context, to, link string) error
	SendVerification(ctx context.Context, to, link string) error
}

//go:embed templates/*.tmpl
var templateFS embed.FS

var (
	htmlLayout = htmltmpl.Must(htmltmpl.ParseFS(templateFS, "templates/layout.html.tmpl"))
	textLayout = texttmpl.Must(texttmpl.ParseFS(templateFS, "templates/layout.txt.tmpl"))
)

// appName / appURL brand the templates. appURL is overridden from config so the
// footer links to the deployed frontend.
const appName = "Seven Spade"

// message holds the per-email copy fed into the shared layout.
type message struct {
	Subject     string
	Preheader   string
	Heading     string
	Intro       string
	ButtonLabel string
	Footnote    string
}

// templateData is the full data set passed to both layouts.
type templateData struct {
	message
	Link    string
	AppName string
	AppURL  string
	Year    int
}

var (
	passwordResetMsg = message{
		Subject:     "Reset your Seven Spade password",
		Preheader:   "Reset your password — this link expires in 15 minutes.",
		Heading:     "Reset your password",
		Intro:       "We received a request to reset your Seven Spade password. Tap the button below to choose a new one. This link expires in 15 minutes and can be used once.",
		ButtonLabel: "Reset password",
		Footnote:    "If you didn't request this, you can safely ignore this email — your password won't change.",
	}
	verificationMsg = message{
		Subject:     "Verify your Seven Spade email",
		Preheader:   "Confirm your email to secure your account.",
		Heading:     "Verify your email",
		Intro:       "Confirm your email address to secure your Seven Spade account and unlock account recovery. This link expires in 24 hours.",
		ButtonLabel: "Verify email",
		Footnote:    "If you didn't create a Seven Spade account, you can safely ignore this email.",
	}
)

// render produces the HTML and plaintext bodies for a message + link.
func render(msg message, link, appURL string) (html, text string, err error) {
	data := templateData{
		message: msg,
		Link:    link,
		AppName: appName,
		AppURL:  appURL,
		Year:    time.Now().Year(),
	}
	var hbuf bytes.Buffer
	if err := htmlLayout.Execute(&hbuf, data); err != nil {
		return "", "", fmt.Errorf("email: render html: %w", err)
	}
	var tbuf bytes.Buffer
	if err := textLayout.Execute(&tbuf, data); err != nil {
		return "", "", fmt.Errorf("email: render text: %w", err)
	}
	return hbuf.String(), tbuf.String(), nil
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

// Encryption selects the SMTP transport security policy.
type Encryption string

const (
	// EncryptionAuto picks implicit TLS for port 465 and STARTTLS otherwise.
	EncryptionAuto     Encryption = "auto"
	EncryptionTLS      Encryption = "tls"      // implicit TLS (SMTPS, usually :465)
	EncryptionSTARTTLS Encryption = "starttls" // explicit STARTTLS (usually :587)
	EncryptionNone     Encryption = "none"     // plaintext (dev/local only)
)

// SMTPSender sends mail through an SMTP server via go-mail.
type SMTPSender struct {
	Host       string
	Port       int
	User       string
	Pass       string
	From       string // envelope + header From address
	FromName   string // optional display name on the From header
	ReplyTo    string // optional Reply-To address
	Encryption Encryption
	AppURL     string // frontend base URL used in the email footer
}

// tlsPolicy maps the configured Encryption (resolving "auto" by port) to a
// go-mail TLS policy and whether SSL/implicit TLS should be on.
func (s SMTPSender) tlsPolicy() (useSSL bool, policy mail.TLSPolicy) {
	enc := s.Encryption
	if enc == "" || enc == EncryptionAuto {
		if s.Port == 465 {
			enc = EncryptionTLS
		} else {
			enc = EncryptionSTARTTLS
		}
	}
	switch enc {
	case EncryptionTLS:
		return true, mail.TLSMandatory
	case EncryptionNone:
		return false, mail.NoTLS
	case EncryptionSTARTTLS:
		return false, mail.TLSMandatory
	default:
		return false, mail.TLSOpportunistic
	}
}

func (s SMTPSender) send(ctx context.Context, to string, msg message, link string) error {
	html, text, err := render(msg, link, s.AppURL)
	if err != nil {
		return err
	}

	m := mail.NewMsg()
	if s.FromName != "" {
		if err := m.FromFormat(s.FromName, s.From); err != nil {
			return fmt.Errorf("email: set from: %w", err)
		}
	} else if err := m.From(s.From); err != nil {
		return fmt.Errorf("email: set from: %w", err)
	}
	if err := m.To(to); err != nil {
		return fmt.Errorf("email: set to: %w", err)
	}
	if s.ReplyTo != "" {
		if err := m.ReplyTo(s.ReplyTo); err != nil {
			return fmt.Errorf("email: set reply-to: %w", err)
		}
	}
	m.Subject(msg.Subject)
	m.SetBodyString(mail.TypeTextPlain, text)
	m.AddAlternativeString(mail.TypeTextHTML, html)

	useSSL, policy := s.tlsPolicy()
	opts := []mail.Option{
		mail.WithPort(s.Port),
		mail.WithTLSPolicy(policy),
		mail.WithTimeout(15 * time.Second),
	}
	if useSSL {
		opts = append(opts, mail.WithSSLPort(false))
	}
	if s.User != "" {
		opts = append(opts, mail.WithSMTPAuth(mail.SMTPAuthPlain), mail.WithUsername(s.User), mail.WithPassword(s.Pass))
	}

	client, err := mail.NewClient(s.Host, opts...)
	if err != nil {
		return fmt.Errorf("email: new client: %w", err)
	}
	if err := client.DialAndSendWithContext(ctx, m); err != nil {
		return fmt.Errorf("email: send to %s: %w", to, err)
	}
	return nil
}

func (s SMTPSender) SendPasswordReset(ctx context.Context, to, link string) error {
	return s.send(ctx, to, passwordResetMsg, link)
}

func (s SMTPSender) SendVerification(ctx context.Context, to, link string) error {
	return s.send(ctx, to, verificationMsg, link)
}

// Config is the subset of app config the email package needs.
type Config struct {
	SMTPHost       string
	SMTPPort       int
	SMTPUser       string
	SMTPPass       string
	SMTPFrom       string
	SMTPFromName   string
	SMTPReplyTo    string
	SMTPEncryption string
	AppURL         string // frontend base URL (FRONTEND_URL) used in the footer
}

// NewFromConfig returns an SMTPSender when SMTP_HOST is configured, otherwise a
// LogSender (dev default — no SMTP server required).
func NewFromConfig(cfg Config) Sender {
	if strings.TrimSpace(cfg.SMTPHost) == "" {
		log.Printf("email: SMTP_HOST not set, using dev log sender (links printed to console)")
		return LogSender{}
	}
	appURL := strings.TrimRight(cfg.AppURL, "/")
	if appURL == "" {
		appURL = "https://sevenspade.local"
	}
	return SMTPSender{
		Host:       cfg.SMTPHost,
		Port:       cfg.SMTPPort,
		User:       cfg.SMTPUser,
		Pass:       cfg.SMTPPass,
		From:       cfg.SMTPFrom,
		FromName:   cfg.SMTPFromName,
		ReplyTo:    cfg.SMTPReplyTo,
		Encryption: Encryption(strings.ToLower(strings.TrimSpace(cfg.SMTPEncryption))),
		AppURL:     appURL,
	}
}
