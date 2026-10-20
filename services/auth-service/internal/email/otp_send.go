package email

import (
	"bytes"
)

type OtpData struct {
	Code      string
	Digits    []string
	ExpiresIn string
}

func (s *Sender) SendOTP(to, code, expiresAt string) error {
	digits := make([]string, 0, len(code))
	for _, r := range code {
		digits = append(digits, string(r))
	}
	data := OtpData{
		Code:      code,
		Digits:    digits,
		ExpiresIn: expiresAt,
	}

	var body bytes.Buffer
	if err := s.otpTmpl.Execute(&body, data); err != nil {
		return err
	}

	m, err := s.compose(to, code, body.String())
	if err != nil {
		return err
	}

	d := s.dial()

	return d.DialAndSend(m)
}
