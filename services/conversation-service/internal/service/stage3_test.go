package service_test

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

func strPtr(s string) *string { return &s }

func TestStage3ExtendedEntitiesValidation(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	send := func(content string, entities ...models.Entity) (*models.Message, error) {
		return e.msgs.Send(ctx, a, service.SendMessageInput{RecipientID: &b, Content: content, Entities: entities})
	}

	// валидный набор новых типов entities
	bID := b.String()
	text := "strike code pre spoiler quote link https://example.com @bob"
	//       012345 7890 123 16..22  24..28 30..33 35..53            55..58
	in := []models.Entity{
		{Type: models.EntityStrikethrough, Offset: 0, Length: 6},
		{Type: models.EntityCode, Offset: 7, Length: 4},
		{Type: models.EntityPre, Offset: 12, Length: 3, Language: strPtr("go")},
		{Type: models.EntitySpoiler, Offset: 16, Length: 7},
		{Type: models.EntityBlockquote, Offset: 24, Length: 5},
		{Type: models.EntityTextLink, Offset: 30, Length: 4, URL: strPtr("https://chavo.example/docs")},
		{Type: models.EntityURL, Offset: 35, Length: 19},
		{Type: models.EntityMention, Offset: 55, Length: 4, UserID: &bID},
	}
	msg, err := send(text, in...)
	if err != nil {
		t.Fatalf("send extended entities: %v", err)
	}
	if len(msg.Entities) != len(in) {
		t.Fatalf("entities len = %d, want %d: %+v", len(msg.Entities), len(in), msg.Entities)
	}
	loaded, err := e.msgs.Get(ctx, b, msg.ID)
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	if len(loaded.Entities) != len(in) || loaded.Entities[5].URL == nil || *loaded.Entities[5].URL != "https://chavo.example/docs" {
		t.Fatalf("loaded entities mismatch: %+v", loaded.Entities)
	}
	if loaded.Entities[2].Language == nil || *loaded.Entities[2].Language != "go" {
		t.Fatalf("pre language not persisted: %+v", loaded.Entities[2])
	}
	if loaded.Entities[7].UserID == nil || *loaded.Entities[7].UserID != bID {
		t.Fatalf("mention user_id not persisted: %+v", loaded.Entities[7])
	}

	// невалидные URL и атрибуты должны отклоняться
	badCases := map[string]struct {
		content  string
		entities []models.Entity
	}{
		"javascript text_link": {
			content:  "click",
			entities: []models.Entity{{Type: models.EntityTextLink, Offset: 0, Length: 5, URL: strPtr("javascript:alert(1)")}},
		},
		"data text_link": {
			content:  "click",
			entities: []models.Entity{{Type: models.EntityTextLink, Offset: 0, Length: 5, URL: strPtr("data:text/html,<h1>x</h1>")}},
		},
		"missing url on text_link": {
			content:  "click",
			entities: []models.Entity{{Type: models.EntityTextLink, Offset: 0, Length: 5}},
		},
		"non-http url entity": {
			content:  "javascript:alert(1)",
			entities: []models.Entity{{Type: models.EntityURL, Offset: 0, Length: 19}},
		},
		"invalid mention uuid": {
			content:  "@bob",
			entities: []models.Entity{{Type: models.EntityMention, Offset: 0, Length: 4, UserID: strPtr("not-a-uuid")}},
		},
		"unexpected url on bold": {
			content:  "hello",
			entities: []models.Entity{{Type: models.EntityBold, Offset: 0, Length: 5, URL: strPtr("https://example.com")}},
		},
		"invalid pre language": {
			content:  "code",
			entities: []models.Entity{{Type: models.EntityPre, Offset: 0, Length: 4, Language: strPtr("go; rm -rf")}},
		},
	}
	for name, tc := range badCases {
		if _, err := send(tc.content, tc.entities...); !errors.Is(err, apperror.ErrIncorrectData) {
			t.Errorf("%s: got %v, want ErrIncorrectData", name, err)
		}
	}
}

func TestStage3UnreadMentionsLifecycle(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	admin, bob, carol := uuid.New(), uuid.New(), uuid.New()

	grp, err := e.groups.Create(ctx, admin, service.CreateGroupInput{
		Name:       "Team",
		Visibility: models.VisibilityPrivate,
		MemberIDs:  []uuid.UUID{bob, carol},
	})
	if err != nil {
		t.Fatal(err)
	}
	convID := grp.Conversation.ID
	bobID := bob.String()
	carolID := carol.String()

	// сообщение 1 упоминает bob
	m1, err := e.msgs.Send(ctx, admin, service.SendMessageInput{
		ConversationID: &convID,
		Content:        "Привет @bob",
		Entities:       []models.Entity{{Type: models.EntityMention, Offset: 7, Length: 4, UserID: &bobID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	// сообщение 2 упоминает bob и carol
	m2, err := e.msgs.Send(ctx, admin, service.SendMessageInput{
		ConversationID: &convID,
		Content:        "@bob @carol гляньте",
		Entities: []models.Entity{
			{Type: models.EntityMention, Offset: 0, Length: 4, UserID: &bobID},
			{Type: models.EntityMention, Offset: 5, Length: 6, UserID: &carolID},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// само-упоминание не должно попадать в непрочитанные
	if _, err = e.msgs.Send(ctx, bob, service.SendMessageInput{
		ConversationID: &convID,
		Content:        "@bob заметка",
		Entities:       []models.Entity{{Type: models.EntityMention, Offset: 0, Length: 4, UserID: &bobID}},
	}); err != nil {
		t.Fatal(err)
	}

	bobSummary, err := e.convs.Get(ctx, bob, convID)
	if err != nil {
		t.Fatal(err)
	}
	if bobSummary.UnreadMentionsCount != 2 {
		t.Fatalf("bob unread_mentions_count = %d, want 2", bobSummary.UnreadMentionsCount)
	}
	ids, err := e.msgs.ListUnreadMentions(ctx, bob, convID, 50)
	if err != nil || !slices.Equal(ids, []uuid.UUID{m1.ID, m2.ID}) {
		t.Fatalf("bob unread mentions = %v %v, want [%s %s]", ids, err, m1.ID, m2.ID)
	}

	// правка m2 убирает упоминание bob -> остаётся только m1
	if _, err = e.msgs.Edit(ctx, admin, m2.ID, "@carol глянь", []models.Entity{
		{Type: models.EntityMention, Offset: 0, Length: 6, UserID: &carolID},
	}); err != nil {
		t.Fatal(err)
	}
	ids, err = e.msgs.ListUnreadMentions(ctx, bob, convID, 50)
	if err != nil || !slices.Equal(ids, []uuid.UUID{m1.ID}) {
		t.Fatalf("after edit bob unread mentions = %v %v, want [%s]", ids, err, m1.ID)
	}

	// прочтение m1 обнуляет счётчик упоминаний у bob
	if err = e.msgs.MarkRead(ctx, bob, convID, m1.ID); err != nil {
		t.Fatal(err)
	}
	bobSummary, _ = e.convs.Get(ctx, bob, convID)
	if bobSummary.UnreadMentionsCount != 0 {
		t.Fatalf("after MarkRead bob unread_mentions_count = %d, want 0", bobSummary.UnreadMentionsCount)
	}

	// удаление m2 обнуляет счётчик упоминаний у carol
	if err = e.msgs.Delete(ctx, admin, m2.ID); err != nil {
		t.Fatal(err)
	}
	carolSummary, _ := e.convs.Get(ctx, carol, convID)
	if carolSummary.UnreadMentionsCount != 0 {
		t.Fatalf("after Delete carol unread_mentions_count = %d, want 0", carolSummary.UnreadMentionsCount)
	}
}

func TestStage3LinkPreviewPersistenceAndValidation(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()

	lp := &models.LinkPreview{
		URL:         "https://example.com/article",
		Title:       strPtr("Заголовок"),
		Description: strPtr("Описание"),
		ImageURL:    strPtr("https://example.com/og.png"),
		SiteName:    strPtr("Example"),
	}
	m, err := e.msgs.Send(ctx, a, service.SendMessageInput{
		RecipientID: &b,
		Content:     "Смотри https://example.com/article",
		LinkPreview: lp,
	})
	if err != nil {
		t.Fatalf("send with link preview: %v", err)
	}
	if m.LinkPreview == nil || m.LinkPreview.URL != lp.URL || *m.LinkPreview.Title != "Заголовок" {
		t.Fatalf("returned link_preview mismatch: %+v", m.LinkPreview)
	}

	got, err := e.msgs.Get(ctx, b, m.ID)
	if err != nil || got.LinkPreview == nil || *got.LinkPreview.ImageURL != "https://example.com/og.png" {
		t.Fatalf("loaded link_preview mismatch: %+v %v", got, err)
	}

	// правка без превью очищает link_preview
	edited, err := e.msgs.Edit(ctx, a, m.ID, "Без ссылки", nil, nil)
	if err != nil || edited.LinkPreview != nil {
		t.Fatalf("edit clearing preview: %+v %v", edited, err)
	}

	// небезопасный URL в превью отклоняется
	if _, err := e.msgs.Send(ctx, a, service.SendMessageInput{
		RecipientID: &b,
		Content:     "bad",
		LinkPreview: &models.LinkPreview{URL: "javascript:alert(1)", Title: strPtr("x")},
	}); !errors.Is(err, apperror.ErrIncorrectData) {
		t.Fatalf("javascript preview url: got %v, want ErrIncorrectData", err)
	}
}

func TestStage3EntitiesAndLinkPreviewEncryptedAtRest(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()

	secretURL := "https://secret.example/docs?token=super-secret-42"
	secretTitle := "Секретный документ"
	m, err := e.msgs.Send(ctx, a, service.SendMessageInput{
		RecipientID: &b,
		Content:     "ссылка тут",
		Entities: []models.Entity{
			{Type: models.EntityTextLink, Offset: 0, Length: 6, URL: strPtr(secretURL)},
		},
		LinkPreview: &models.LinkPreview{
			URL:      secretURL,
			Title:    strPtr(secretTitle),
			ImageURL: strPtr("https://secret.example/img.png?sig=abc"),
		},
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	// в таблице messages не должно быть открытого текста ни в entities, ни в *_enc
	var (
		rawEntities       []byte
		entitiesEnc       []byte
		linkPreviewEnc    []byte
		rawOutboxPayloads []byte
	)
	if err := e.pool.QueryRow(ctx,
		`SELECT entities::text::bytea, entities_enc, link_preview_enc FROM messages WHERE id = $1`, m.ID,
	).Scan(&rawEntities, &entitiesEnc, &linkPreviewEnc); err != nil {
		t.Fatalf("query raw message: %v", err)
	}
	if len(rawEntities) != 0 {
		t.Fatalf("legacy plaintext entities column must be NULL, got %s", rawEntities)
	}
	if len(entitiesEnc) == 0 || len(linkPreviewEnc) == 0 {
		t.Fatalf("encrypted columns must be populated: entities_enc=%d link_preview_enc=%d", len(entitiesEnc), len(linkPreviewEnc))
	}
	for _, needle := range [][]byte{[]byte("secret.example"), []byte("super-secret-42"), []byte(secretTitle)} {
		if bytes.Contains(entitiesEnc, needle) || bytes.Contains(linkPreviewEnc, needle) {
			t.Fatalf("plaintext %q leaked into encrypted message columns", needle)
		}
	}

	// в outbox_events URL и заголовок также зашифрованы до публикации
	if err := e.pool.QueryRow(ctx,
		`SELECT payload::text::bytea FROM outbox_events WHERE topic = 'message-events' ORDER BY id DESC LIMIT 1`,
	).Scan(&rawOutboxPayloads); err != nil {
		t.Fatalf("query outbox: %v", err)
	}
	for _, needle := range [][]byte{[]byte("secret.example"), []byte("super-secret-42"), []byte(secretTitle)} {
		if bytes.Contains(rawOutboxPayloads, needle) {
			t.Fatalf("plaintext %q leaked into outbox_events: %s", needle, rawOutboxPayloads)
		}
	}

	// чтение возвращает расшифрованные entities и link_preview
	got, err := e.msgs.Get(ctx, b, m.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.Entities) != 1 || got.Entities[0].URL == nil || *got.Entities[0].URL != secretURL {
		t.Fatalf("decrypted entities mismatch: %+v", got.Entities)
	}
	if got.LinkPreview == nil || got.LinkPreview.URL != secretURL || *got.LinkPreview.Title != secretTitle {
		t.Fatalf("decrypted link_preview mismatch: %+v", got.LinkPreview)
	}

	// при мягком удалении зашифрованные колонки очищаются
	if err := e.msgs.Delete(ctx, a, m.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := e.pool.QueryRow(ctx,
		`SELECT entities_enc, link_preview_enc FROM messages WHERE id = $1`, m.ID,
	).Scan(&entitiesEnc, &linkPreviewEnc); err != nil {
		t.Fatalf("query deleted message: %v", err)
	}
	if len(entitiesEnc) != 0 || len(linkPreviewEnc) != 0 {
		t.Fatalf("deleted message must clear encrypted columns: entities_enc=%d link_preview_enc=%d", len(entitiesEnc), len(linkPreviewEnc))
	}
}
