package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"strings"
	"time"
)

const (
	totpPeriod       = 30
	totpDigits       = 6
	totpSecretBytes  = 20
	totpWindowSteps  = 1
	backupAlphabet   = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	backupCodeLength = 8
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateTOTPSecret создаёт криптографически стойкий секрет в Base32 (без padding).
func GenerateTOTPSecret() (string, error) {
	buf := make([]byte, totpSecretBytes)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", fmt.Errorf("read totp secret: %w", err)
	}
	return b32.EncodeToString(buf), nil
}

// BuildOTPAuthURL собирает otpauth:// URI для QR-кода приложения-аутентификатора.
func BuildOTPAuthURL(issuer, account, secret string) string {
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)
	params := url.Values{}
	params.Set("secret", secret)
	params.Set("issuer", issuer)
	params.Set("algorithm", "SHA1")
	params.Set("digits", "6")
	params.Set("period", "30")
	return fmt.Sprintf("otpauth://totp/%s?%s", label, params.Encode())
}

// GenerateTOTPCode вычисляет 6-значный код по RFC 6238 (HMAC-SHA1, шаг 30 с).
func GenerateTOTPCode(secret string, at time.Time) (string, error) {
	return generateTOTPForStep(secret, at.Unix()/totpPeriod)
}

func generateTOTPForStep(secret string, step int64) (string, error) {
	key, err := b32.DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return "", fmt.Errorf("decode totp secret: %w", err)
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))

	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	code := (uint32(sum[offset]&0x7f) << 24) |
		(uint32(sum[offset+1]) << 16) |
		(uint32(sum[offset+2]) << 8) |
		uint32(sum[offset+3])
	return fmt.Sprintf("%0*d", totpDigits, code%1_000_000), nil
}

// ValidateTOTP проверяет 6-значный код в окне ±1 шаг (90 с) и возвращает совпавший временной шаг.
func ValidateTOTP(secret, code string, now time.Time) (int64, bool) {
	clean := strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(clean) != totpDigits {
		return 0, false
	}
	for i := 0; i < len(clean); i++ {
		if clean[i] < '0' || clean[i] > '9' {
			return 0, false
		}
	}
	cur := now.Unix() / totpPeriod
	for delta := int64(-totpWindowSteps); delta <= totpWindowSteps; delta++ {
		step := cur + delta
		expected, err := generateTOTPForStep(secret, step)
		if err != nil {
			return 0, false
		}
		if hmac.Equal([]byte(clean), []byte(expected)) {
			return step, true
		}
	}
	return 0, false
}

// EncryptAESGCM шифрует данные в формате [nonce 12B][ciphertext][tag 16B].
func EncryptAESGCM(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptAESGCM расшифровывает данные формата [nonce 12B][ciphertext][tag 16B].
func DecryptAESGCM(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce, body := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, body, nil)
}

// GenerateBackupCodes создаёт n одноразовых резервных кодов вида XXXX-XXXX.
func GenerateBackupCodes(n int) ([]string, error) {
	codes := make([]string, 0, n)
	seen := make(map[string]struct{}, n)
	maxIdx := big.NewInt(int64(len(backupAlphabet)))
	for len(codes) < n {
		var b strings.Builder
		b.Grow(backupCodeLength + 1)
		for i := 0; i < backupCodeLength; i++ {
			if i == 4 {
				b.WriteByte('-')
			}
			idx, err := rand.Int(rand.Reader, maxIdx)
			if err != nil {
				return nil, fmt.Errorf("generate backup code: %w", err)
			}
			b.WriteByte(backupAlphabet[idx.Int64()])
		}
		code := b.String()
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}
	return codes, nil
}

// NormalizeBackupCode приводит резервный код к единому виду (без дефисов и пробелов, верхний регистр).
func NormalizeBackupCode(code string) string {
	s := strings.ToUpper(strings.TrimSpace(code))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

// HashBackupCode возвращает SHA-256 от нормализованного резервного кода.
func HashBackupCode(code string) string {
	return HashSHA256(NormalizeBackupCode(code))
}
