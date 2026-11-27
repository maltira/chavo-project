package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/auth-service/config"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/utils"
)

type stubUserRepo struct {
	repository.UserRepository
	users map[uuid.UUID]*models.User
}

func (r *stubUserRepo) FindByID(_ context.Context, id uuid.UUID) (*models.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, apperror.ErrNotFound
	}
	return u, nil
}

type stubBackupRepo struct {
	hashes map[string]bool
}

func (r *stubBackupRepo) ReplaceAllTx(context.Context, pgx.Tx, uuid.UUID, []string) error { return nil }
func (r *stubBackupRepo) DeleteByUserIDTx(context.Context, pgx.Tx, uuid.UUID) error       { return nil }
func (r *stubBackupRepo) CountRemaining(context.Context, uuid.UUID) (int, error)          { return len(r.hashes), nil }
func (r *stubBackupRepo) Consume(_ context.Context, _ uuid.UUID, codeHash string) (bool, error) {
	if !r.hashes[codeHash] {
		return false, nil
	}
	delete(r.hashes, codeHash)
	return true, nil
}

func TestSetupAndConfirmTOTPRejectedWhenAlreadyEnabled(t *testing.T) {
	uid := uuid.New()
	users := &stubUserRepo{
		users: map[uuid.UUID]*models.User{
			uid: {
				ID:              uid,
				Email:           "user@example.com",
				TwoFactorMethod: models.TwoFactorTOTP,
			},
		},
	}
	svc := NewAuthService(users, nil, nil, nil, nil, nil, nil, nil, nil, nil, &config.Config{}, zap.NewNop())

	if _, _, err := svc.SetupTOTP(context.Background(), uid, "Secret123!"); !errors.Is(err, apperror.ErrTOTPAlreadyEnabled) {
		t.Fatalf("SetupTOTP on enabled TOTP = %v, want ErrTOTPAlreadyEnabled", err)
	}
	if _, err := svc.ConfirmTOTP(context.Background(), uid, "123456"); !errors.Is(err, apperror.ErrTOTPAlreadyEnabled) {
		t.Fatalf("ConfirmTOTP on enabled TOTP = %v, want ErrTOTPAlreadyEnabled", err)
	}
}

func TestBackupCodeWorksEvenWhenTOTPSecretCannotDecrypt(t *testing.T) {
	uid := uuid.New()
	oldKey := make([]byte, 32)
	newKey := make([]byte, 32)
	oldKey[0], newKey[0] = 1, 2

	enc, err := utils.EncryptAESGCM(oldKey, []byte("JBSWY3DPEHPK3PXP"))
	if err != nil {
		t.Fatal(err)
	}
	backupCode := "ABCD-EFGH"
	backups := &stubBackupRepo{hashes: map[string]bool{utils.HashBackupCode(backupCode): true}}
	otpSvc := NewOtpService(nil, nil, backups, newKey, nil, zap.NewNop())

	user := &models.User{
		ID:              uid,
		TwoFactorMethod: models.TwoFactorTOTP,
		TOTPSecretEnc:   enc,
	}

	ok, err := otpSvc.VerifyTOTPCode(context.Background(), user, backupCode, true)
	if err != nil || !ok {
		t.Fatalf("backup code must succeed despite rotated TOTP key: ok=%v err=%v", ok, err)
	}
	// повторное использование того же резервного кода должно быть отклонено
	ok, err = otpSvc.VerifyTOTPCode(context.Background(), user, backupCode, true)
	if err != nil || ok {
		t.Fatalf("reused backup code must fail: ok=%v err=%v", ok, err)
	}
}

type deletedSaveTokens struct {
	memTokens
	rejectSave bool
}

func (m *deletedSaveTokens) Save(ctx context.Context, t *models.RefreshToken) error {
	if m.rejectSave {
		return apperror.ErrUnauthorized
	}
	return m.memTokens.Save(ctx, t)
}

func TestGenerateTokensRejectedForDeletedUser(t *testing.T) {
	repo := &deletedSaveTokens{
		memTokens:  memTokens{rows: map[uuid.UUID]*models.RefreshToken{}},
		rejectSave: true,
	}
	svc := newTestTokenService(&repo.memTokens)
	_ = svc
	tokenSvc := NewTokenService(repo, nil, nil, &config.Config{JWTSecret: "test"}, zap.NewNop())

	if _, err := tokenSvc.GenerateTokens(context.Background(), uuid.New(), "", "", ""); !errors.Is(err, apperror.ErrUnauthorized) {
		t.Fatalf("GenerateTokens for deleted user = %v, want ErrUnauthorized", err)
	}
}
