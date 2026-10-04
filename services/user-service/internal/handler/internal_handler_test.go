package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/handler"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/service"
)

type fakeBlocks struct {
	service.BlockService
	blocked map[[2]uuid.UUID]bool // {blocker, blocked}
}

func (f fakeBlocks) GetBlockStatus(_ context.Context, my, target uuid.UUID) (bool, bool, error) {
	return f.blocked[[2]uuid.UUID{my, target}], f.blocked[[2]uuid.UUID{target, my}], nil
}

type fakeProfiles struct {
	service.ProfileService
	exist map[uuid.UUID]bool
}

func (f fakeProfiles) FindByID(_ context.Context, id uuid.UUID) (*models.Profile, error) {
	if !f.exist[id] {
		return nil, apperror.ErrNotFound
	}
	return &models.Profile{UserID: id}, nil
}

type fakeSettings struct {
	service.SettingsService
	invites map[uuid.UUID]bool // отсутствие записи = пользователя нет
}

func (f fakeSettings) GetSettings(_ context.Context, id uuid.UUID) (*models.Settings, error) {
	allowed, ok := f.invites[id]
	if !ok {
		return nil, apperror.ErrNotFound
	}
	return &models.Settings{UserID: id, AllowGroupInvites: allowed}, nil
}

func newEngine(blocks fakeBlocks, profiles fakeProfiles, settings fakeSettings) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := handler.NewInternalHandler(blocks, profiles, settings, zap.NewNop())
	r := gin.New()
	r.GET("/internal/messaging-allowed", h.MessagingAllowed)
	r.GET("/internal/users/:user_id/exists", h.UserExists)
	r.GET("/internal/users/:user_id/group-invite-allowed", h.GroupInviteAllowed)
	return r
}

func get(t *testing.T, r http.Handler, path string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

func TestGroupInviteAllowed(t *testing.T) {
	inviter, open, closed, blockedByInviter, blocksInviter, ghost := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	r := newEngine(
		fakeBlocks{blocked: map[[2]uuid.UUID]bool{
			{inviter, blockedByInviter}: true, // приглашающий заблокировал приглашаемого
			{blocksInviter, inviter}:    true, // приглашаемый заблокировал приглашающего
		}},
		fakeProfiles{},
		fakeSettings{invites: map[uuid.UUID]bool{open: true, closed: false, blockedByInviter: true, blocksInviter: true}},
	)

	cases := map[string]struct {
		path     string
		wantCode int
		allowed  bool
	}{
		"open settings, no block":     {"/internal/users/" + open.String() + "/group-invite-allowed?inviter=" + inviter.String(), 200, true},
		"invites disabled":            {"/internal/users/" + closed.String() + "/group-invite-allowed?inviter=" + inviter.String(), 200, false},
		"inviter blocked the user":    {"/internal/users/" + blockedByInviter.String() + "/group-invite-allowed?inviter=" + inviter.String(), 200, false},
		"user blocked the inviter":    {"/internal/users/" + blocksInviter.String() + "/group-invite-allowed?inviter=" + inviter.String(), 200, false},
		"inviter missing":             {"/internal/users/" + open.String() + "/group-invite-allowed", 400, false},
		"inviter is not a uuid":       {"/internal/users/" + open.String() + "/group-invite-allowed?inviter=nope", 400, false},
		"user id is not a uuid":       {"/internal/users/nope/group-invite-allowed?inviter=" + inviter.String(), 400, false},
		"user without settings (404)": {"/internal/users/" + ghost.String() + "/group-invite-allowed?inviter=" + inviter.String(), 404, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			code, body := get(t, r, tc.path)
			if code != tc.wantCode {
				t.Fatalf("status = %d, want %d (%v)", code, tc.wantCode, body)
			}
			if code == 200 && body["allowed"] != tc.allowed {
				t.Fatalf("allowed = %v, want %v", body["allowed"], tc.allowed)
			}
		})
	}
}

func TestMessagingAllowed(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	r := newEngine(fakeBlocks{blocked: map[[2]uuid.UUID]bool{{a, b}: true}}, fakeProfiles{}, fakeSettings{})

	code, body := get(t, r, "/internal/messaging-allowed?sender="+a.String()+"&recipient="+b.String())
	if code != 200 || body["allowed"] != false || body["blocked_by_sender"] != true || body["blocked_by_recipient"] != false {
		t.Fatalf("sender blocked recipient: %d %v", code, body)
	}
	code, body = get(t, r, "/internal/messaging-allowed?sender="+b.String()+"&recipient="+a.String())
	if code != 200 || body["allowed"] != false || body["blocked_by_sender"] != false || body["blocked_by_recipient"] != true {
		t.Fatalf("recipient blocked sender: %d %v", code, body)
	}
	if code, body = get(t, r, "/internal/messaging-allowed?sender="+a.String()+"&recipient="+c.String()); code != 200 || body["allowed"] != true {
		t.Fatalf("no block: %d %v", code, body)
	}
	if code, _ = get(t, r, "/internal/messaging-allowed?sender="+a.String()); code != 400 {
		t.Fatalf("missing recipient: %d", code)
	}
}

func TestUserExists(t *testing.T) {
	known, unknown := uuid.New(), uuid.New()
	r := newEngine(fakeBlocks{}, fakeProfiles{exist: map[uuid.UUID]bool{known: true}}, fakeSettings{})

	if code, body := get(t, r, "/internal/users/"+known.String()+"/exists"); code != 200 || body["exists"] != true {
		t.Fatalf("known: %d %v", code, body)
	}
	if code, body := get(t, r, "/internal/users/"+unknown.String()+"/exists"); code != 200 || body["exists"] != false {
		t.Fatalf("unknown: %d %v", code, body)
	}
	if code, _ := get(t, r, "/internal/users/nope/exists"); code != 400 {
		t.Fatalf("bad uuid: %d", code)
	}
}
