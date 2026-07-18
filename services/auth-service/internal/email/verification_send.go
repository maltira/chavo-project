package email

import (
	"bytes"

	"github.com/go-gomail/gomail"
)

type VerificationData struct {
	VerifyURL string
	ExpiresIn string // "15 минут"
}

func (s *Sender) SendVerification(to, verifyURL string) error {
	data := VerificationData{
		VerifyURL: verifyURL,
		ExpiresIn: "15 минут",
	}

	var body bytes.Buffer
	if err := s.verificationTmpl.Execute(&body, data); err != nil {
		return err
	}

	m := gomail.NewMessage()
	m.SetHeader("From", s.user)
	m.SetHeader("To", to)
	m.SetHeader("Subject", "Подтвердите ваш аккаунт на Chavo?")
	m.SetBody("text/html", body.String())

	d := s.dial()

	return d.DialAndSend(m)
}
