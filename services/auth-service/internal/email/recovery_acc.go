package email

import (
	"bytes"

	"github.com/go-gomail/gomail"
)

type RecoveryData struct {
	RecoveryURL string
	ExpiresIn   string
}

func (s *Sender) SendRecovery(to, recoveryURL string, expiresIn string) error {
	data := RecoveryData{
		RecoveryURL: recoveryURL,
		ExpiresIn:   expiresIn,
	}

	var body bytes.Buffer
	if err := s.recoveryTmpl.Execute(&body, data); err != nil {
		return err
	}

	m := gomail.NewMessage()
	m.SetHeader("From", s.user)
	m.SetHeader("To", to)
	m.SetHeader("Subject", "Восстановление аккаунта")
	m.SetBody("text/html", body.String())

	d := s.dial()

	return d.DialAndSend(m)
}
