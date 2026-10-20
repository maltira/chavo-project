package email

import (
	"bytes"
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

	m, err := s.compose(to, "Подтвердите ваш аккаунт на Chavo", body.String())
	if err != nil {
		return err
	}

	d := s.dial()

	return d.DialAndSend(m)
}
