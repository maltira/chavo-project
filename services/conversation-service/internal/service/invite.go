package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/conversation-service/pkg/crypto"
)

const (
	inviteCodeLength   = 12
	inviteCodeAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	inviteCodeAttempts = 5
)

// slugPattern - код ссылки открытой группы, задаётся админом
var slugPattern = regexp.MustCompile(`^[a-zA-Z0-9_]{3,32}$`)

// inviteCode - сгенерированный код ссылки закрытой группы: сам код, SHA-256 для поиска и шифротекст для админов
type inviteCode struct {
	code string
	hash string
	enc  []byte
}

// newInviteCode - 12 символов [a-z0-9] из crypto/rand (~62 бита)
func newInviteCode() (string, error) {
	b := make([]byte, inviteCodeLength)
	max := big.NewInt(int64(len(inviteCodeAlphabet)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generate invite code: %w", err)
		}
		b[i] = inviteCodeAlphabet[n.Int64()]
	}
	return string(b), nil
}

// hashInviteToken - коды закрытых групп в нижнем регистре, поэтому ссылка работает при любом регистре
func hashInviteToken(token string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(token)))
	return hex.EncodeToString(sum[:])
}

// issueInviteCode выпускает код, не совпадающий ни с одним кодом открытой группы: коды обоих видов живут в одной ссылке /chat/<код>
func issueInviteCode(ctx context.Context, convs repository.ConversationRepository, q repository.DBTX, cipher *crypto.Cipher) (*inviteCode, error) {
	for range inviteCodeAttempts {
		code, err := newInviteCode()
		if err != nil {
			return nil, err
		}
		if _, err = convs.FindBySlug(ctx, q, code); err == nil {
			continue
		} else if !errors.Is(err, apperror.ErrNotFound) {
			return nil, err
		}
		enc, err := cipher.Encrypt([]byte(code))
		if err != nil {
			return nil, fmt.Errorf("encrypt invite code: %w", err)
		}
		return &inviteCode{code: code, hash: hashInviteToken(code), enc: enc}, nil
	}
	return nil, errors.New("issue invite code: too many collisions")
}

// validateSlug проверяет формат и что код не совпадает с кодом какой-нибудь закрытой группы;
// совпадение с другой открытой группой ловит уникальный индекс (ErrSlugTaken)
func validateSlug(ctx context.Context, convs repository.ConversationRepository, q repository.DBTX, slug string) (string, error) {
	s := strings.TrimSpace(slug)
	if s == "" {
		return "", apperror.ErrSlugRequired
	}
	if !slugPattern.MatchString(s) {
		return "", apperror.ErrSlugInvalid
	}
	if _, err := convs.FindByInviteHash(ctx, q, hashInviteToken(s)); err == nil {
		return "", apperror.ErrSlugTaken
	} else if !errors.Is(err, apperror.ErrNotFound) {
		return "", err
	}
	return s, nil
}

// decryptInviteCode - nil, если у группы нет кода (открытая)
func decryptInviteCode(cipher *crypto.Cipher, enc []byte) (*string, error) {
	if enc == nil {
		return nil, nil
	}
	plain, err := cipher.Decrypt(enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt invite code: %w", err)
	}
	code := string(plain)
	return &code, nil
}
