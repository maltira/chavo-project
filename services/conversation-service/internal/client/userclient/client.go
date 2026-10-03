package userclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
)

// Client — синхронные обращения к internal-API user-service.
type Client interface {
	// CheckMessagingAllowed возвращает ErrBlockedByMe / ErrBlockedByThem (с точки зрения sender).
	CheckMessagingAllowed(ctx context.Context, sender, recipient uuid.UUID) error
	UserExists(ctx context.Context, userID uuid.UUID) (bool, error)
	GroupInviteAllowed(ctx context.Context, userID uuid.UUID) (bool, error)
}

type httpClient struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) Client {
	return &httpClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 3 * time.Second},
	}
}

func (c *httpClient) CheckMessagingAllowed(ctx context.Context, sender, recipient uuid.UUID) error {
	q := url.Values{"sender": {sender.String()}, "recipient": {recipient.String()}}
	var resp struct {
		Allowed            bool `json:"allowed"`
		BlockedBySender    bool `json:"blocked_by_sender"`
		BlockedByRecipient bool `json:"blocked_by_recipient"`
	}
	if _, err := c.get(ctx, "/internal/messaging-allowed?"+q.Encode(), &resp); err != nil {
		return err
	}
	switch {
	case resp.BlockedBySender:
		return apperror.ErrBlockedByMe
	case resp.BlockedByRecipient, !resp.Allowed:
		return apperror.ErrBlockedByThem
	}
	return nil
}

func (c *httpClient) UserExists(ctx context.Context, userID uuid.UUID) (bool, error) {
	var resp struct {
		Exists bool `json:"exists"`
	}
	if _, err := c.get(ctx, "/internal/users/"+userID.String()+"/exists", &resp); err != nil {
		return false, err
	}
	return resp.Exists, nil
}

func (c *httpClient) GroupInviteAllowed(ctx context.Context, userID uuid.UUID) (bool, error) {
	var resp struct {
		Allowed bool `json:"allowed"`
	}
	status, err := c.get(ctx, "/internal/users/"+userID.String()+"/group-invite-allowed", &resp)
	if status == http.StatusNotFound {
		return false, apperror.ErrUserNotFound
	}
	if err != nil {
		return false, err
	}
	return resp.Allowed, nil
}

// get выполняет GET и декодирует JSON при 200; для остальных статусов возвращает ErrUserServiceError.
func (c *httpClient) get(ctx context.Context, path string, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return 0, fmt.Errorf("%w: build request: %v", apperror.ErrUserServiceError, err)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", apperror.ErrUserServiceError, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return res.StatusCode, fmt.Errorf("%w: status %d", apperror.ErrUserServiceError, res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return res.StatusCode, fmt.Errorf("%w: decode: %v", apperror.ErrUserServiceError, err)
	}
	return res.StatusCode, nil
}
