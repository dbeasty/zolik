package auth

import (
	"context"
	"fmt"
	"log"
	"net/smtp"
	"regexp"
	"strings"
	"sync"
)

// Mail is one outbound message.
type Mail struct {
	To      string
	Subject string
	Text    string
}

// Mailer sends mail.
//
// An interface with a logging default rather than a hard SMTP dependency,
// because email sign-in has to be usable the moment somebody clones the repo:
// requiring a configured mail provider to run the app locally would make the
// login path the one feature nobody tests.
type Mailer interface {
	Send(ctx context.Context, m Mail) error
}

// LogMailer writes messages to the server log instead of sending them. It is
// the default when no SMTP host is configured, which makes local development
// and end-to-end tests work with no external service: the code is right there
// in the terminal.
//
// It refuses to be used outside local development — see NewMailer — because a
// production deployment silently logging its login codes is far worse than one
// that fails to start.
type LogMailer struct{}

func (LogMailer) Send(_ context.Context, m Mail) error {
	log.Printf("mail (not sent — no SMTP configured)\n  to: %s\n  subject: %s\n  %s",
		m.To, m.Subject, strings.ReplaceAll(strings.TrimSpace(m.Text), "\n", "\n  "))
	return nil
}

// SMTPConfig is the mail transport configuration.
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
	// FromName is the display name on the From header.
	FromName string
}

// SMTPMailer sends over SMTP with STARTTLS, which is what every mainstream
// transactional provider (SES, Postmark, Resend, Mailgun) speaks.
type SMTPMailer struct {
	cfg SMTPConfig
}

// NewMailer picks a mailer for the environment: SMTP when a host is
// configured, the logging stand-in otherwise.
//
// local reports whether this is a development environment. When it is false
// and no SMTP host is set, this returns an error rather than a mailer — a
// deployment that cannot send mail cannot offer email sign-in, and finding
// that out at startup is much cheaper than finding it out from a player who
// never received their code.
func NewMailer(cfg SMTPConfig, local bool) (Mailer, error) {
	if strings.TrimSpace(cfg.Host) == "" {
		if !local {
			return nil, fmt.Errorf("email sign-in is enabled but SMTP_HOST is not set")
		}
		return LogMailer{}, nil
	}
	if cfg.Port == "" {
		cfg.Port = "587"
	}
	if cfg.From == "" {
		return nil, fmt.Errorf("SMTP_FROM is required when SMTP_HOST is set")
	}
	return &SMTPMailer{cfg: cfg}, nil
}

func (s *SMTPMailer) Send(ctx context.Context, m Mail) error {
	addr := s.cfg.Host + ":" + s.cfg.Port
	from := s.cfg.From
	header := from
	if s.cfg.FromName != "" {
		header = fmt.Sprintf("%s <%s>", s.cfg.FromName, from)
	}

	// Header values are assembled from configuration and from an address that
	// has already been validated, but CR/LF is stripped anyway: a newline
	// reaching a header is SMTP header injection, and the check costs nothing.
	msg := []byte(
		"From: " + stripCRLF(header) + "\r\n" +
			"To: " + stripCRLF(m.To) + "\r\n" +
			"Subject: " + stripCRLF(m.Subject) + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=UTF-8\r\n" +
			"\r\n" + m.Text)

	var authMech smtp.Auth
	if s.cfg.Username != "" {
		authMech = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	// net/smtp has no context support, so the deadline is honoured by running
	// the send on its own goroutine and abandoning the result if the caller
	// gives up first. The send still completes in the background — mail is
	// idempotent enough that a delivered message after a timed-out request is
	// the better failure.
	done := make(chan error, 1)
	go func() {
		done <- smtp.SendMail(addr, authMech, from, []string{m.To}, msg)
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func stripCRLF(s string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(s)
}

// CapturingMailer wraps another Mailer and additionally remembers the code
// from the most recent mail sent to each address, so it can be read back
// through GET /auth/dev/last-code — see Handlers.devLastCode.
//
// This exists purely to make the passwordless flow *testable* end to end
// without a real inbox: a browser-driven test can request a code and has no
// way to "read the terminal" LogMailer writes to. It is wired in only when
// app.Config.TestEndpointsEnabled is set (see app.go and the same gate
// game.GameRestHandlers' debugState uses) — a real deployment never
// constructs one, so a live sign-in code is never held in process memory
// outside of local/e2e use.
type CapturingMailer struct {
	underlying Mailer

	mu   sync.Mutex
	last map[string]string // normalised address -> code
}

func NewCapturingMailer(underlying Mailer) *CapturingMailer {
	return &CapturingMailer{underlying: underlying, last: map[string]string{}}
}

var sixDigitCode = regexp.MustCompile(`\b(\d{6})\b`)

func (m *CapturingMailer) Send(ctx context.Context, mail Mail) error {
	if code := sixDigitCode.FindStringSubmatch(mail.Text); code != nil {
		m.mu.Lock()
		m.last[NormalizeEmail(mail.To)] = code[1]
		m.mu.Unlock()
	}
	return m.underlying.Send(ctx, mail)
}

// LastCode returns the most recently mailed code for an address, if any.
func (m *CapturingMailer) LastCode(email string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	code, ok := m.last[NormalizeEmail(email)]
	return code, ok
}
