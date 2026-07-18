package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// GenerateAccessToken creates a signed JWT access token with a JTI claim.
func GenerateAccessToken(userID uuid.UUID, secret string, duration time.Duration) (token string, jti string, err error) {
	jti = uuid.NewString()
	now := time.Now()

	claims := jwt.MapClaims{
		"sub": userID.String(),
		"jti": jti,
		"iat": now.Unix(),
		"exp": now.Add(duration).Unix(),
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err = t.SignedString([]byte(secret))
	if err != nil {
		return "", "", fmt.Errorf("sign access token: %w", err)
	}
	return token, jti, nil
}

// GenerateRefreshToken creates a cryptographically random refresh token string.
func GenerateRefreshToken(duration time.Duration) (token string, expiresAt time.Time, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", time.Time{}, fmt.Errorf("generate refresh token: %w", err)
	}
	return hex.EncodeToString(b), time.Now().Add(duration), nil
}

// GenerateSecureToken creates a random hex-encoded token of the given byte length.
func GenerateSecureToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HashSHA256 returns the hex-encoded SHA-256 hash of the input.
func HashSHA256(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
