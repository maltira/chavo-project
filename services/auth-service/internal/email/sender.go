package email

import (
	"crypto/tls"
	"embed"
	"html/template"
	"io"
	"strconv"

	"github.com/go-gomail/gomail"
	"go.uber.org/zap"
)

//go:embed templates/*
var templateFS embed.FS

// логотип со светлой надписью (письма только в тёмной теме): вкладывается в письмо вложением
// и подставляется как cid:<имя файла>, поэтому показывается без внешнего хостинга
// и без загрузки картинок из сети - Gmail вырезает inline-svg и data:-ссылки
//
//go:embed assets/*.png
var assetFS embed.FS

var logoFiles = []string{"logo-on-dark.png"}

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

// compose собирает письмо: HTML-тело и вложенный логотип
func (s *Sender) compose(to, subject, body string) (*gomail.Message, error) {
	m := gomail.NewMessage()
	m.SetHeader("From", s.user)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	for _, name := range logoFiles {
		data, err := assetFS.ReadFile("assets/" + name)
		if err != nil {
			return nil, err
		}
		m.Embed(name, gomail.SetCopyFunc(func(w io.Writer) error {
			_, err := w.Write(data)
			return err
		}))
	}
	return m, nil
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

// mustParseTemplate собирает письмо из общего каркаса layout.html и страницы path:
// страница задаёт блоки title, preheader, heading и content; Execute запускает каркас
func mustParseTemplate(path string) *template.Template {
	tmpl, err := template.ParseFS(templateFS, "templates/layout.html", path)
	if err != nil {
		panic("failed to parse email template " + path + ": " + err.Error())
	}
	return tmpl
}
