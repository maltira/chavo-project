package email

import (
	"bytes"

	"github.com/go-gomail/gomail"
)

type OtpData struct {
	Code      string
	ExpiresIn string
}

func (s *Sender) SendOTP(to, code, expiresAt string) error {
	data := OtpData{
		Code:      code,
		ExpiresIn: expiresAt,
	}

	var body bytes.Buffer
	if err := s.otpTmpl.Execute(&body, data); err != nil {
		return err
	}

	m := gomail.NewMessage()
	m.SetHeader("From", s.user)
	m.SetHeader("To", to)
	m.SetHeader("Subject", "Ваш код подтверждения — "+code)
	m.SetBody("text/html", body.String())

	d := s.dial()

	return d.DialAndSend(m)
}
