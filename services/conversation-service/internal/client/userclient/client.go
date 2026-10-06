package userclient

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	userv1 "github.com/maltira/chavo-project-backend/proto/gen/go/user/v1"
	"github.com/maltira/chavo-project-backend/proto/grpcx"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
)

const callTimeout = 3 * time.Second

// Client — синхронные обращения к internal-API user-service.
type Client interface {
	// CheckMessagingAllowed возвращает ErrBlockedByMe / ErrBlockedByThem (с точки зрения sender).
	CheckMessagingAllowed(ctx context.Context, sender, recipient uuid.UUID) error
	UserExists(ctx context.Context, userID uuid.UUID) (bool, error)
	// GroupInviteAllowed: false, если пользователь запретил приглашения или между ним и inviter есть блокировка.
	GroupInviteAllowed(ctx context.Context, userID, inviterID uuid.UUID) (bool, error)
}

type grpcClient struct {
	api userv1.UserInternalServiceClient
}

// New — клиент поверх gRPC-соединения с user-service.
func New(conn grpc.ClientConnInterface) Client {
	return &grpcClient{api: userv1.NewUserInternalServiceClient(conn)}
}

func unavailable(err error) error {
	return fmt.Errorf("%w: %v", apperror.ErrUserServiceError, err)
}

func (c *grpcClient) CheckMessagingAllowed(ctx context.Context, sender, recipient uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	resp, err := c.api.MessagingAllowed(ctx, &userv1.MessagingAllowedRequest{SenderId: sender.String(), RecipientId: recipient.String()})
	if err != nil {
		return unavailable(err)
	}
	switch {
	case resp.GetBlockedBySender():
		return apperror.ErrBlockedByMe
	case resp.GetBlockedByRecipient(), !resp.GetAllowed():
		return apperror.ErrBlockedByThem
	}
	return nil
}

func (c *grpcClient) UserExists(ctx context.Context, userID uuid.UUID) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	resp, err := c.api.UserExists(ctx, &userv1.UserExistsRequest{UserId: userID.String()})
	if err != nil {
		return false, unavailable(err)
	}
	return resp.GetExists(), nil
}

func (c *grpcClient) GroupInviteAllowed(ctx context.Context, userID, inviterID uuid.UUID) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	resp, err := c.api.GroupInviteAllowed(ctx, &userv1.GroupInviteAllowedRequest{UserId: userID.String(), InviterId: inviterID.String()})
	if err != nil {
		if code, _, _ := grpcx.Details(err); code == codes.NotFound {
			return false, apperror.ErrUserNotFound
		}
		return false, unavailable(err)
	}
	return resp.GetAllowed(), nil
}
