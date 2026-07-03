// Package mailer sends transactional email over SMTP using only the standard
// library. It supports both implicit TLS (port 465, e.g. Timeweb) and STARTTLS
// (port 587). When no password is configured the mailer is disabled and Send is
// a no-op, so local/dev environments run without SMTP credentials.
package mailer

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

const dialTimeout = 15 * time.Second

// Mailer holds SMTP connection settings.
type Mailer struct {
	host     string
	port     int
	username string
	password string
	from     string // full "Name <addr>" form used in the From header
	fromAddr string // bare address used as the SMTP envelope sender
	useSSL   bool   // implicit TLS (465) vs STARTTLS (other ports)
}

// New builds a Mailer. host/port/username/from fall back to Timeweb defaults.
// A blank password yields a disabled mailer (Enabled() == false).
func New(host string, port int, username, password, from string) *Mailer {
	if host == "" {
		host = "smtp.timeweb.ru"
	}
	if port == 0 {
		port = 465
	}
	if username == "" {
		username = "support@tyrax.tech"
	}
	if from == "" {
		from = "TYRAX <" + username + ">"
	}
	fromAddr := username
	if parsed, err := mail.ParseAddress(from); err == nil {
		fromAddr = parsed.Address
	}
	return &Mailer{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
		fromAddr: fromAddr,
		useSSL:   port == 465,
	}
}

// Enabled reports whether SMTP credentials are configured.
func (m *Mailer) Enabled() bool {
	return m != nil && m.password != ""
}

// Send delivers a multipart/alternative message (plain text + HTML) to one
// recipient. It returns nil (no-op) when the mailer is disabled.
func (m *Mailer) Send(to, subject, textBody, htmlBody string) error {
	if !m.Enabled() {
		slog.Warn("mailer disabled: skipping send", slog.String("to", to), slog.String("subject", subject))
		return nil
	}

	msg := m.buildMessage(to, subject, textBody, htmlBody)
	addr := net.JoinHostPort(m.host, fmt.Sprintf("%d", m.port))
	auth := smtp.PlainAuth("", m.username, m.password, m.host)

	client, err := m.dial(addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer client.Close()

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := client.Mail(m.fromAddr); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close body: %w", err)
	}
	return client.Quit()
}

// dial opens an authenticated SMTP client, choosing implicit TLS or STARTTLS.
func (m *Mailer) dial(addr string) (*smtp.Client, error) {
	tlsCfg := &tls.Config{ServerName: m.host, MinVersion: tls.VersionTLS12}

	if m.useSSL {
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: dialTimeout}, "tcp", addr, tlsCfg)
		if err != nil {
			return nil, err
		}
		return smtp.NewClient(conn, m.host)
	}

	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return nil, err
	}
	client, err := smtp.NewClient(conn, m.host)
	if err != nil {
		return nil, err
	}
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(tlsCfg); err != nil {
			client.Close()
			return nil, err
		}
	}
	return client, nil
}

func (m *Mailer) buildMessage(to, subject, textBody, htmlBody string) []byte {
	boundary := "tyrax_boundary_x7f3a9"
	var b strings.Builder
	b.WriteString("From: " + m.from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + encodeHeader(subject) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n")

	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(textBody + "\r\n\r\n")

	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(htmlBody + "\r\n\r\n")

	b.WriteString("--" + boundary + "--\r\n")
	return []byte(b.String())
}

// encodeHeader RFC 2047-encodes a UTF-8 subject so non-ASCII renders correctly.
func encodeHeader(s string) string {
	return mime.QEncoding.Encode("UTF-8", s)
}
