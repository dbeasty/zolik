package auth

import (
	"context"
	"fmt"
	"log"
	"net/smtp"
	"strings"
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
