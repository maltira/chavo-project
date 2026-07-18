package utils

import (
	"crypto/rand"
	"math/big"
)

// GenerateOTP creates a cryptographically random 6-digit OTP code.
func GenerateOTP() (string, error) {
	const digits = "0123456789"
	code := make([]byte, 6)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		code[i] = digits[n.Int64()]
	}
	return string(code), nil
}
