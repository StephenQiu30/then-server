// Package mail sends account messages through an authenticated TLS SMTP connection.
package mail

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"mime"
	"net"
	mailaddr "net/mail"
	"net/smtp"
	"strings"
	"time"
)

var ErrDelivery = errors.New("mail delivery unavailable")

type Sender struct {
	address  string
	host     string
	from     string
	authCode string
	roots    *x509.CertPool
}

func NewSender(address, from, authCode string) (*Sender, error) {
	host, port, err := net.SplitHostPort(address)
	parsed, addressErr := mailaddr.ParseAddress(from)
	if err != nil || host == "" || port == "" || addressErr != nil || parsed.Address != from || authCode == "" || strings.ContainsAny(authCode, "\r\n") {
		return nil, ErrDelivery
	}
	return &Sender{address: address, host: host, from: from, authCode: authCode}, nil
}

func (s *Sender) Send(ctx context.Context, recipient, subject, body string) error {
	parsed, err := mailaddr.ParseAddress(recipient)
	if s == nil || err != nil || parsed.Address != recipient || strings.ContainsAny(subject, "\r\n") {
		return ErrDelivery
	}
	dialer := net.Dialer{Timeout: 5 * time.Second}
	plain, err := dialer.DialContext(ctx, "tcp", s.address)
	if err != nil {
		return ErrDelivery
	}
	defer plain.Close()
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(10 * time.Second)
	}
	if plain.SetDeadline(deadline) != nil {
		return ErrDelivery
	}
	connection := tls.Client(plain, &tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12, RootCAs: s.roots})
	if connection.HandshakeContext(ctx) != nil {
		return ErrDelivery
	}
	client, err := smtp.NewClient(connection, s.host)
	if err != nil {
		return ErrDelivery
	}
	defer client.Close()
	if client.Auth(smtp.PlainAuth("", s.from, s.authCode, s.host)) != nil || client.Mail(s.from) != nil || client.Rcpt(recipient) != nil {
		return ErrDelivery
	}
	writer, err := client.Data()
	if err != nil {
		return ErrDelivery
	}
	message := "From: " + s.from + "\r\nTo: " + recipient + "\r\nSubject: " + mime.QEncoding.Encode("utf-8", subject) +
		"\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + strings.ReplaceAll(body, "\n", "\r\n")
	if _, err := io.WriteString(writer, message); err != nil {
		_ = writer.Close()
		return ErrDelivery
	}
	if writer.Close() != nil || client.Quit() != nil {
		return ErrDelivery
	}
	return nil
}
