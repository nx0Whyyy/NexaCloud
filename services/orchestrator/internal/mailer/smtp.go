package mailer

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

type Config struct {
	Host, Username, Password, From, FromName string
	Port                                     int
}

type Service struct{ config Config }

func New(config Config) *Service { return &Service{config: config} }

func (s *Service) Enabled() bool { return s.config.Host != "" && s.config.From != "" }

func (s *Service) SendVerification(to, username, link string) error {
	if !s.Enabled() {
		return nil
	}
	if strings.ContainsAny(to+s.config.From+s.config.FromName, "\r\n") {
		return fmt.Errorf("invalid SMTP header")
	}
	subject := "Vérifiez votre adresse NexaCloud"
	body := fmt.Sprintf("Bonjour %s,\r\n\r\nConfirmez votre adresse e-mail pour activer votre compte NexaCloud :\r\n%s\r\n\r\nCe lien expire dans 30 minutes. Si vous n'êtes pas à l'origine de cette demande, ignorez cet e-mail.\r\n", username, link)
	message := []byte("From: " + s.config.FromName + " <" + s.config.From + ">\r\nTo: " + to + "\r\nSubject: " + subject + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body)
	address := net.JoinHostPort(s.config.Host, fmt.Sprint(s.config.Port))
	connection, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		return err
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(20 * time.Second))
	client, err := smtp.NewClient(connection, s.config.Host)
	if err != nil {
		return err
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return fmt.Errorf("SMTP server does not offer STARTTLS")
	} else if err := client.StartTLS(&tls.Config{ServerName: s.config.Host, MinVersion: tls.VersionTLS12}); err != nil {
		return err
	}
	if s.config.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)); err != nil {
			return err
		}
	}
	if err := client.Mail(s.config.From); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(message); err != nil {
		return err
	}
	return w.Close()
}

func NormalizePublicURL(value string) string { return strings.TrimRight(value, "/") }
