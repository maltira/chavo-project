package userclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
)

func TestCheckMessagingAllowed(t *testing.T) {
	cases := map[string]struct {
		body string
		want error
	}{
		"allowed":      {`{"allowed":true,"blocked_by_sender":false,"blocked_by_recipient":false}`, nil},
		"by sender":    {`{"allowed":false,"blocked_by_sender":true,"blocked_by_recipient":false}`, apperror.ErrBlockedByMe},
		"by recipient": {`{"allowed":false,"blocked_by_sender":false,"blocked_by_recipient":true}`, apperror.ErrBlockedByThem},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/internal/messaging-allowed" || r.URL.Query().Get("sender") == "" {
					t.Errorf("unexpected request %s", r.URL)
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			err := New(srv.URL).CheckMessagingAllowed(context.Background(), uuid.New(), uuid.New())
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestUserServiceFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := New(srv.URL).UserExists(context.Background(), uuid.New())
	if !errors.Is(err, apperror.ErrUserServiceError) {
		t.Fatalf("got %v", err)
	}
}

func TestGroupInviteAllowedNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := New(srv.URL).GroupInviteAllowed(context.Background(), uuid.New())
	if !errors.Is(err, apperror.ErrUserNotFound) {
		t.Fatalf("got %v", err)
	}
}
