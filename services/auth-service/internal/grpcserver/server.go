// Package grpcserver — gRPC-транспорт auth-service. Cookie и тексты успешных ответов — забота Gateway.
package grpcserver

import (
	"context"
	"net/mail"
	"unicode/utf8"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	authv1 "github.com/maltira/chavo-project-backend/proto/gen/go/auth/v1"
	"github.com/maltira/chavo-project-backend/proto/grpcx"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/service"
	"github.com/maltira/chavo-project-backend/services/auth-service/internal/utils"
)

const (
	minPasswordLength = 8
	maxPasswordLength = 24
	otpCodeLength     = 6
)

type Server struct {
	authv1.UnimplementedAuthServiceServer

	auth   service.AuthService
	otp    service.OtpService
	tokens service.TokenService
}

func New(auth service.AuthService, otp service.OtpService, tokens service.TokenService) *Server {
	return &Server{auth: auth, otp: otp, tokens: tokens}
}

func currentUser(ctx context.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(grpcx.IncomingUserID(ctx))
	if err != nil {
		return uuid.Nil, apperror.ErrUnauthorized
	}
	return id, nil
}

func validEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email
}

func validPassword(p string, checkMax bool) bool {
	n := utf8.RuneCountInString(p)
	return n >= minPasswordLength && (!checkMax || n <= maxPasswordLength)
}

func tokenPair(t *service.Tokens) *authv1.TokenPair {
	return &authv1.TokenPair{
		AccessToken:      t.AccessToken,
		RefreshToken:     t.RefreshToken,
		RefreshExpiresAt: timestamppb.New(t.ExpiresAt),
		UserId:           t.UserID.String(),
		SessionId:        t.SessionID.String(),
	}
}

func (s *Server) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	if !validEmail(req.GetEmail()) || !validPassword(req.GetPassword(), true) {
		return nil, apperror.ErrIncorrectData
	}
	if err := s.auth.Register(ctx, req.GetEmail(), req.GetPassword()); err != nil {
		return nil, err
	}
	return &authv1.RegisterResponse{}, nil
}

func (s *Server) VerifyRegister(ctx context.Context, req *authv1.VerifyRegisterRequest) (*authv1.VerifyRegisterResponse, error) {
	if req.GetToken() == "" {
		return nil, apperror.ErrIncorrectData
	}
	if err := s.auth.VerifyRegister(ctx, req.GetToken()); err != nil {
		return nil, err
	}
	return &authv1.VerifyRegisterResponse{}, nil
}

func (s *Server) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	if !validEmail(req.GetEmail()) || !validPassword(req.GetPassword(), false) {
		return nil, apperror.ErrIncorrectData
	}
	challengeID, err := s.auth.Login(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, err
	}
	return &authv1.LoginResponse{ChallengeId: challengeID}, nil
}

func (s *Server) VerifyOTP(ctx context.Context, req *authv1.VerifyOTPRequest) (*authv1.VerifyOTPResponse, error) {
	if req.GetChallengeId() == "" || utf8.RuneCountInString(req.GetCode()) != otpCodeLength {
		return nil, apperror.ErrIncorrectData
	}
	userID, err := s.otp.VerifyChallenge(ctx, req.GetChallengeId(), req.GetCode())
	if err != nil {
		return nil, err
	}
	ua := req.GetClient().GetUserAgent()
	tokens, err := s.tokens.GenerateTokens(ctx, userID, req.GetClient().GetIp(), ua, utils.ParseDeviceInfo(ua))
	if err != nil {
		return nil, err
	}
	return &authv1.VerifyOTPResponse{Tokens: tokenPair(tokens)}, nil
}

func (s *Server) ResendOTP(ctx context.Context, req *authv1.ResendOTPRequest) (*authv1.ResendOTPResponse, error) {
	if req.GetChallengeId() == "" {
		return nil, apperror.ErrIncorrectData
	}
	if err := s.otp.ResendChallenge(ctx, req.GetChallengeId()); err != nil {
		return nil, err
	}
	return &authv1.ResendOTPResponse{}, nil
}

func (s *Server) Refresh(ctx context.Context, req *authv1.RefreshRequest) (*authv1.RefreshResponse, error) {
	if req.GetRefreshToken() == "" {
		return nil, apperror.ErrUnauthorized
	}
	ua := req.GetClient().GetUserAgent()
	tokens, err := s.tokens.Refresh(ctx, req.GetRefreshToken(), req.GetClient().GetIp(), ua, utils.ParseDeviceInfo(ua))
	if err != nil {
		return nil, err
	}
	return &authv1.RefreshResponse{Tokens: tokenPair(tokens)}, nil
}

func (s *Server) Logout(ctx context.Context, req *authv1.LogoutRequest) (*authv1.LogoutResponse, error) {
	if err := s.tokens.RevokeByToken(ctx, req.GetRefreshToken()); err != nil {
		return nil, err
	}
	return &authv1.LogoutResponse{}, nil
}

func (s *Server) RequestPasswordReset(ctx context.Context, req *authv1.RequestPasswordResetRequest) (*authv1.RequestPasswordResetResponse, error) {
	if !validEmail(req.GetEmail()) {
		return nil, apperror.ErrIncorrectData
	}
	if err := s.auth.RequestPasswordReset(ctx, req.GetEmail()); err != nil {
		return nil, err
	}
	return &authv1.RequestPasswordResetResponse{}, nil
}

func (s *Server) ConfirmPasswordReset(ctx context.Context, req *authv1.ConfirmPasswordResetRequest) (*authv1.ConfirmPasswordResetResponse, error) {
	if req.GetToken() == "" || !validPassword(req.GetPassword(), true) {
		return nil, apperror.ErrIncorrectData
	}
	if err := s.auth.ConfirmPasswordReset(ctx, req.GetToken(), req.GetPassword()); err != nil {
		return nil, err
	}
	return &authv1.ConfirmPasswordResetResponse{}, nil
}

func (s *Server) ChangePassword(ctx context.Context, req *authv1.ChangePasswordRequest) (*authv1.ChangePasswordResponse, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetCurrentPassword() == "" || !validPassword(req.GetNewPassword(), true) {
		return nil, apperror.ErrIncorrectData
	}
	if err = s.auth.ChangePassword(ctx, userID, req.GetCurrentPassword(), req.GetNewPassword()); err != nil {
		return nil, err
	}
	return &authv1.ChangePasswordResponse{}, nil
}

func (s *Server) ListSessions(ctx context.Context, _ *authv1.ListSessionsRequest) (*authv1.ListSessionsResponse, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	sessions, err := s.tokens.ListActiveSessions(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]*authv1.Session, len(sessions))
	for i, rt := range sessions {
		out[i] = &authv1.Session{
			Id:         rt.ID.String(),
			DeviceName: rt.DeviceName,
			UserAgent:  rt.UserAgent,
			IpAddress:  rt.IPAddress,
			CreatedAt:  timestamppb.New(rt.CreatedAt),
			ExpiresAt:  timestamppb.New(rt.ExpiresAt),
		}
	}
	return &authv1.ListSessionsResponse{Sessions: out}, nil
}

func (s *Server) TerminateSession(ctx context.Context, req *authv1.TerminateSessionRequest) (*authv1.TerminateSessionResponse, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	sessionID, err := uuid.Parse(req.GetSessionId())
	if err != nil {
		return nil, apperror.ErrIncorrectData
	}
	if err = s.tokens.RevokeByID(ctx, userID, sessionID); err != nil {
		return nil, err
	}
	return &authv1.TerminateSessionResponse{}, nil
}

func (s *Server) ResolveSession(ctx context.Context, req *authv1.ResolveSessionRequest) (*authv1.ResolveSessionResponse, error) {
	userID, sessionID, err := s.tokens.Resolve(ctx, req.GetRefreshToken())
	if err != nil {
		return nil, err
	}
	return &authv1.ResolveSessionResponse{UserId: userID.String(), SessionId: sessionID.String()}, nil
}
