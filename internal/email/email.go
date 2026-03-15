package email

import (
	"fmt"
	"log"
	"net/smtp"
	"strings"
)

// Config holds SMTP configuration
type Config struct {
	Host     string
	Port     int
	From     string
	Username string
	Password string
}

// Service sends emails via SMTP
type Service struct {
	config Config
}

// NewService creates a new email service
func NewService(cfg Config) *Service {
	return &Service{config: cfg}
}

// IsConfigured returns true if SMTP settings are provided
func (s *Service) IsConfigured() bool {
	return s.config.Host != ""
}

// SendPasswordReset sends a password reset email or logs the link if SMTP is not configured
func (s *Service) SendPasswordReset(toEmail, resetURL string) error {
	if !s.IsConfigured() {
		log.Printf("[DEV] Password reset link for %s: %s", toEmail, resetURL)
		return nil
	}

	body := s.buildResetEmail(toEmail, resetURL)
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	var auth smtp.Auth
	if s.config.Username != "" {
		auth = smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
	}

	return smtp.SendMail(addr, auth, s.config.From, []string{toEmail}, []byte(body))
}

func (s *Service) buildResetEmail(to, resetURL string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("To: %s\r\n", to))
	b.WriteString(fmt.Sprintf("From: %s\r\n", s.config.From))
	b.WriteString("Subject: Password Reset - hCTF2\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(fmt.Sprintf(`<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml">
<head>
  <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
</head>
<body style="font-family: sans-serif; color: #e0e0e0; margin: 0; padding: 0;">
  <table width="100%%" border="0" cellpadding="0" cellspacing="0" bgcolor="#1a1a2e">
    <tr>
      <td style="padding: 20px 0;">
        <table width="500" border="0" cellpadding="0" cellspacing="0" align="center" bgcolor="#16213e" style="border-radius: 8px;">
          <tr>
            <td style="padding: 30px;">
              <h2 style="color: #a55eea; margin-top: 0;">Password Reset</h2>
              <p style="color: #e0e0e0;">You requested a password reset for your hCTF2 account (%s).</p>
              <p style="color: #e0e0e0;">Click the link below to reset your password. This link expires in 30 minutes.</p>
              <table width="100%%" border="0" cellpadding="0" cellspacing="0">
                <tr>
                  <td align="center" style="padding: 25px 0;">
                    <table border="0" cellpadding="0" cellspacing="0">
                      <tr>
                        <td align="center" bgcolor="#a55eea" style="border-radius: 6px;">
                          <a href="%s" style="color: #ffffff; font-family: sans-serif; font-size: 14px; font-weight: bold; text-decoration: none; padding: 12px 24px; display: block;">Reset Password</a>
                        </td>
                      </tr>
                    </table>
                  </td>
                </tr>
              </table>
              <p style="color: #888888; font-size: 12px;">If you didn't request this, ignore this email.</p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, to, resetURL))
	return b.String()
}
