package email

import (
	"bytes"
)

type PassResetData struct {
	ResetURL  string
	ExpiresIn string
}

func (s *Sender) SendPasswordReset(to, resetURL string) error {
	data := PassResetData{
		ResetURL:  resetURL,
		ExpiresIn: "15 минут",
	}

	var body bytes.Buffer
	if err := s.passResetTmpl.Execute(&body, data); err != nil {
		return err
	}

	m, err := s.compose(to, "Сброс пароля", body.String())
	if err != nil {
		return err
	}

	d := s.dial()

	return d.DialAndSend(m)
}
