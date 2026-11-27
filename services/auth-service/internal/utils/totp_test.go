package utils

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestTOTPRFC6238Vector(t *testing.T) {
	// стандартный тестовый секрет RFC 6238 ("12345678901234567890" в Base32)
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	code, err := GenerateTOTPCode(secret, time.Unix(59, 0))
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// RFC 6238 SHA1 (8 digits = 94287082, последние 6 цифр = 287082)
	if code != "287082" {
		t.Fatalf("code = %q, want 287082", code)
	}

	step, ok := ValidateTOTP(secret, code, time.Unix(59, 0))
	if !ok || step != 1 {
		t.Fatalf("ValidateTOTP(59s) = (%d, %v), want (1, true)", step, ok)
	}
	// соседний шаг (+30с) допускается
	step, ok = ValidateTOTP(secret, code, time.Unix(89, 0))
	if !ok || step != 1 {
		t.Fatalf("ValidateTOTP(89s) = (%d, %v), want (1, true)", step, ok)
	}
	// через два шага (+60с) уже не подходит
	if _, ok = ValidateTOTP(secret, code, time.Unix(119, 0)); ok {
		t.Fatal("ValidateTOTP(119s) unexpectedly succeeded")
	}
}

func TestGenerateTOTPSecretAndURL(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("secret: %v", err)
	}
	if len(secret) != 32 {
		t.Fatalf("secret len = %d, want 32", len(secret))
	}
	u := BuildOTPAuthURL("Chavo", "user@example.com", secret)
	if !strings.HasPrefix(u, "otpauth://totp/Chavo:user@example.com?") || !strings.Contains(u, "secret="+secret) {
		t.Fatalf("unexpected url: %s", u)
	}
}

func TestAESGCMRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	plain := []byte("JBSWY3DPEHPK3PXP")
	enc, err := EncryptAESGCM(key, plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Equal(enc, plain) {
		t.Fatal("ciphertext equals plaintext")
	}
	dec, err := DecryptAESGCM(key, enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(dec, plain) {
		t.Fatalf("decrypted = %q, want %q", dec, plain)
	}
}

func TestBackupCodes(t *testing.T) {
	codes, err := GenerateBackupCodes(8)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(codes) != 8 {
		t.Fatalf("len = %d, want 8", len(codes))
	}
	if HashBackupCode(codes[0]) != HashBackupCode(strings.ToLower(strings.ReplaceAll(codes[0], "-", " "))) {
		t.Fatal("normalized backup code hash mismatch")
	}
}
