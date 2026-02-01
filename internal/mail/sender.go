package mail

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
)

type MailSender interface {
	Send(to string, subject string, textBody string, htmlBody string) error
}

type ConsoleMailSender struct{}

func (s *ConsoleMailSender) Send(to string, subject string, textBody string, htmlBody string) error {
	log.Printf("=== MOCK EMAIL ===\nTo: %s\nSubject: %s\nText Body: %s\nHTML Body: %s\n==================", to, subject, textBody, htmlBody)
	return nil
}

type SmtpConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
}

type SmtpMailSender struct {
	config SmtpConfig
}

func NewSmtpMailSender(config SmtpConfig) *SmtpMailSender {
	return &SmtpMailSender{config: config}
}

func (s *SmtpMailSender) Send(to string, subject string, textBody string, htmlBody string) error {
	auth := smtp.PlainAuth("", s.config.User, s.config.Password, s.config.Host)
	address := fmt.Sprintf("%s:%s", s.config.Host, s.config.Port)

	// Simple MIME multipart email
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	msg := []byte(fmt.Sprintf("To: %s\r\n"+
		"From: %s\r\n"+
		"Subject: %s\r\n"+
		"%s\r\n"+
		"%s", to, s.config.From, subject, mime, htmlBody))

	// If htmlBody is empty, fall back to textBody?
	// For now implementing simple HTML sending as requested by "htmlBody" usage in plan.
	// A more robust implementation would do multipart/alternative.
	// Given the scope, let's keep it simple but functional for the task.
	if htmlBody == "" {
		mime = "Content-Type: text/plain; charset=\"UTF-8\";\n\n"
		msg = []byte(fmt.Sprintf("To: %s\r\n"+
			"From: %s\r\n"+
			"Subject: %s\r\n"+
			"%s\r\n"+
			"%s", to, s.config.From, subject, mime, textBody))
	}

	err := smtp.SendMail(address, auth, s.config.From, []string{to}, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	return nil
}

func NewSenderFromEnv() MailSender {
	provider := os.Getenv("MAIL_PROVIDER")
	if provider == "smtp" {
		return NewSmtpMailSender(SmtpConfig{
			Host:     os.Getenv("SMTP_HOST"),
			Port:     os.Getenv("SMTP_PORT"),
			User:     os.Getenv("SMTP_USER"),
			Password: os.Getenv("SMTP_PASSWORD"),
			From:     os.Getenv("SMTP_FROM"),
		})
	}
	return &ConsoleMailSender{}
}
