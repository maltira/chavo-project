package config

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func setBaseEnv(t *testing.T, env string) {
	t.Helper()
	t.Setenv("ENV", env)
	t.Setenv("JWT_SECRET", "jwt-secret-1")
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("TOTP_ENCRYPTION_KEY", "")
	t.Setenv("TOTP_ENCRYPTION_KEY_PREV", "")
}

func b64Key(seed byte) string {
	b := make([]byte, 32)
	for i := range b {
		b[i] = seed + byte(i)
	}
	return base64.StdEncoding.EncodeToString(b)
}

func TestLoadRequiresTOTPKeyOutsideDevelopment(t *testing.T) {
	for _, env := range []string{"production", "staging"} {
		t.Run(env, func(t *testing.T) {
			setBaseEnv(t, env)
			if _, err := Load(); err == nil {
				t.Fatalf("Load() with ENV=%s and empty TOTP_ENCRYPTION_KEY must fail", env)
			}
		})
	}
}

func TestLoadAllowsJWTDerivedTOTPKeyInDevelopment(t *testing.T) {
	setBaseEnv(t, "development")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() in development failed: %v", err)
	}
	want := sha256.Sum256([]byte("chavo-totp-v1:jwt-secret-1"))
	if string(cfg.TOTPEncryptionKey) != string(want[:]) {
		t.Fatalf("unexpected dev fallback TOTP key")
	}
}

func TestLoadExplicitTOTPKeyIndependentOfJWTSecret(t *testing.T) {
	key1 := b64Key(1)
	prev1 := b64Key(2)
	prev2 := b64Key(3)

	setBaseEnv(t, "production")
	t.Setenv("TOTP_ENCRYPTION_KEY", key1)
	t.Setenv("TOTP_ENCRYPTION_KEY_PREV", prev1+" , "+prev2)

	cfg1, err := Load()
	if err != nil {
		t.Fatalf("Load() with explicit key failed: %v", err)
	}
	if len(cfg1.TOTPEncryptionPrevKeys) != 2 {
		t.Fatalf("prev keys = %d, want 2", len(cfg1.TOTPEncryptionPrevKeys))
	}

	// смена JWT_SECRET не должна менять ключ шифрования TOTP
	t.Setenv("JWT_SECRET", "jwt-secret-rotated")
	cfg2, err := Load()
	if err != nil {
		t.Fatalf("Load() after JWT rotation failed: %v", err)
	}
	if string(cfg1.TOTPEncryptionKey) != string(cfg2.TOTPEncryptionKey) {
		t.Fatal("TOTPEncryptionKey changed after JWT_SECRET rotation")
	}
}

func TestLoadRejectsInvalidTOTPKeys(t *testing.T) {
	setBaseEnv(t, "production")
	t.Setenv("TOTP_ENCRYPTION_KEY", "not-base64!")
	if _, err := Load(); err == nil {
		t.Fatal("invalid TOTP_ENCRYPTION_KEY must fail")
	}

	t.Setenv("TOTP_ENCRYPTION_KEY", b64Key(1))
	t.Setenv("TOTP_ENCRYPTION_KEY_PREV", base64.StdEncoding.EncodeToString([]byte("short")))
	if _, err := Load(); err == nil {
		t.Fatal("invalid TOTP_ENCRYPTION_KEY_PREV must fail")
	}
}
