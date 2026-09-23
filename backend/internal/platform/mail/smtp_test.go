package mail

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

type smtpReceipt struct {
	username string
	password string
	message  string
}

func TestSenderRequiresVerifiedTLSAndAuthorizationCode(t *testing.T) {
	certificate, roots := testCertificate(t)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal("start synthetic TLS SMTP server")
	}
	defer listener.Close()
	receipts := make(chan smtpReceipt, 1)
	go func() {
		for {
			connection, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go serveSyntheticSMTP(connection, receipts)
		}
	}()
	sender, err := NewSender(listener.Addr().String(), "sender@163.com", "synthetic-authorization-code")
	if err != nil {
		t.Fatal("construct sender")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sender.Send(ctx, "recipient@example.test", "验证邮箱", "一次性验证链接\n"); !errors.Is(err, ErrDelivery) {
		t.Fatal("self-signed SMTP certificate was accepted without trusted roots")
	}
	sender.roots = roots
	if err := sender.Send(ctx, "recipient@example.test", "验证邮箱", "一次性验证链接\n"); err != nil {
		t.Fatal("trusted TLS SMTP delivery failed")
	}
	select {
	case receipt := <-receipts:
		if receipt.username != "sender@163.com" || receipt.password != "synthetic-authorization-code" || !strings.Contains(receipt.message, "Content-Type: text/plain; charset=UTF-8") || !strings.Contains(receipt.message, "一次性验证链接") {
			t.Fatal("SMTP authentication or UTF-8 message differed from contract")
		}
	case <-ctx.Done():
		t.Fatal("synthetic SMTP server did not receive a message")
	}
}

func serveSyntheticSMTP(connection net.Conn, receipts chan<- smtpReceipt) {
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
	reader := bufio.NewReader(connection)
	_, _ = connection.Write([]byte("220 localhost ready\r\n"))
	var receipt smtpReceipt
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "EHLO "):
			_, _ = connection.Write([]byte("250-localhost\r\n250 AUTH PLAIN\r\n"))
		case strings.HasPrefix(line, "AUTH PLAIN "):
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(line, "AUTH PLAIN "))
			if err != nil {
				return
			}
			fields := strings.Split(string(decoded), "\x00")
			if len(fields) != 3 {
				return
			}
			receipt.username, receipt.password = fields[1], fields[2]
			_, _ = connection.Write([]byte("235 authenticated\r\n"))
		case strings.HasPrefix(line, "MAIL FROM:") || strings.HasPrefix(line, "RCPT TO:"):
			_, _ = connection.Write([]byte("250 accepted\r\n"))
		case line == "DATA":
			_, _ = connection.Write([]byte("354 send message\r\n"))
			var message strings.Builder
			for {
				part, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if part == ".\r\n" {
					break
				}
				message.WriteString(part)
			}
			receipt.message = message.String()
			_, _ = connection.Write([]byte("250 queued\r\n"))
		case line == "QUIT":
			_, _ = connection.Write([]byte("221 bye\r\n"))
			receipts <- receipt
			return
		default:
			return
		}
	}
}

func testCertificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal("generate synthetic SMTP key")
	}
	der, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "127.0.0.1"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}, &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "127.0.0.1"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}, &key.PublicKey, key)
	if err != nil {
		t.Fatal("create synthetic SMTP certificate")
	}
	roots := x509.NewCertPool()
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal("parse synthetic SMTP certificate")
	}
	roots.AddCert(certificate)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, roots
}
