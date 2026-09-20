package email

import (
	"bytes"

	"github.com/go-gomail/gomail"
)

type PassResetData struct {
	ResetURL  string
	ExpiresIn string
}

func (s *Sender) SendPasswordReset(to, resetURL string, expiresIn string) error {
	data := PassResetData{
		ResetURL:  resetURL,
		ExpiresIn: expiresIn,
	}

	var body bytes.Buffer
	if err := s.passResetTmpl.Execute(&body, data); err != nil {
		return err
	}

	m := gomail.NewMessage()
	m.SetHeader("From", s.user)
	m.SetHeader("To", to)
	m.SetHeader("Subject", "Сброс пароля")
	m.SetBody("text/html", body.String())

	d := s.dial()

	return d.DialAndSend(m)
}
