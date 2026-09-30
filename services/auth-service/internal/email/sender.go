package email

import (
	"crypto/tls"
	"embed"
	"html/template"
	"strconv"

	"github.com/go-gomail/gomail"
	"go.uber.org/zap"
)

//go:embed templates/*
var templateFS embed.FS

// EmailSender defines the interface for sending emails
type EmailSender interface {
	SendVerification(to, verifyURL string) error
	SendOTP(to, code, expiresAt string) error
	SendPasswordReset(to, resetURL string) error
}

// Sender is a real SMTP implementation of EmailSender using gomail
type Sender struct {
	host string
	port int
	user string
	pass string
	log  *zap.Logger

	verificationTmpl *template.Template
	otpTmpl          *template.Template
	passResetTmpl    *template.Template
}

// NewSender creates a new Sender with SMTP credentials and pre-parsed templates
func NewSender(host, port, user, pass string, log *zap.Logger) EmailSender {
	smtpPort, err := strconv.Atoi(port)
	if err != nil {
		log.Warn("Invalid SMTP port, defaulting to 587", zap.String("port", port), zap.Error(err))
		smtpPort = 587
	}

	s := &Sender{
		host: host,
		port: smtpPort,
		user: user,
		pass: pass,
		log:  log,
	}

	s.verificationTmpl = mustParseTemplate("templates/verification.html")
	s.otpTmpl = mustParseTemplate("templates/otp.html")
	s.passResetTmpl = mustParseTemplate("templates/reset_pass.html")

	return s
}

// dial возвращает настроенный SMTP-диалер с явным TLSConfig
func (s *Sender) dial() *gomail.Dialer {
	d := gomail.NewDialer(s.host, s.port, s.user, s.pass)
	d.TLSConfig = &tls.Config{
		ServerName: s.host,
		MinVersion: tls.VersionTLS12,
	}
	return d
}

func mustParseTemplate(path string) *template.Template {
	tmpl, err := template.ParseFS(templateFS, path)
	if err != nil {
		panic("failed to parse email template " + path + ": " + err.Error())
	}
	return tmpl
}
