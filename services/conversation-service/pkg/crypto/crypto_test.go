package crypto

import (
	"bytes"
	"testing"
)

func TestCipherRoundTrip(t *testing.T) {
	c, err := NewCipher(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}

	enc1, _ := c.Encrypt([]byte("привет"))
	enc2, _ := c.Encrypt([]byte("привет"))
	if bytes.Equal(enc1, enc2) {
		t.Fatal("nonce must differ between encryptions")
	}
	if bytes.Contains(enc1, []byte("привет")) {
		t.Fatal("ciphertext contains plaintext")
	}

	got, err := c.Decrypt(enc1)
	if err != nil || string(got) != "привет" {
		t.Fatalf("decrypt = %q, %v", got, err)
	}

	enc1[len(enc1)-1] ^= 1
	if _, err := c.Decrypt(enc1); err == nil {
		t.Fatal("tampered data must fail")
	}
}

func TestNewCipherBadKey(t *testing.T) {
	if _, err := NewCipher([]byte("short")); err == nil {
		t.Fatal("expected error for short key")
	}
}
