package outbox

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/auth-service/internal/repository"
)

type noopRunner struct{}

func (noopRunner) WithTx(_ context.Context, fn func(tx pgx.Tx) error) error { return fn(nil) }

type fakeOutboxRepo struct {
	locked    bool
	records   []repository.OutboxRecord
	published []int64
	failed    []int64
}

func (r *fakeOutboxRepo) InsertTx(context.Context, pgx.Tx, string, string, any) error { return nil }
func (r *fakeOutboxRepo) TryLockTx(context.Context, pgx.Tx) (bool, error)             { return r.locked, nil }
func (r *fakeOutboxRepo) FetchUnpublishedTx(context.Context, pgx.Tx, int) ([]repository.OutboxRecord, error) {
	return r.records, nil
}
func (r *fakeOutboxRepo) MarkPublishedTx(_ context.Context, _ pgx.Tx, id int64) error {
	r.published = append(r.published, id)
	return nil
}
func (r *fakeOutboxRepo) RecordFailureTx(_ context.Context, _ pgx.Tx, id int64) error {
	r.failed = append(r.failed, id)
	return nil
}
func (r *fakeOutboxRepo) DeletePublishedBefore(context.Context, time.Duration) (int64, error) {
	return 0, nil
}

type fakePub struct {
	failAt int
	sent   []string
}

func (p *fakePub) Publish(_ context.Context, topic, key string, _ any) error {
	if p.failAt > 0 && len(p.sent)+1 == p.failAt {
		return errors.New("kafka unavailable")
	}
	p.sent = append(p.sent, topic+":"+key)
	return nil
}

func TestRunOncePublishesInOrderAndStopsOnKafkaError(t *testing.T) {
	repo := &fakeOutboxRepo{
		locked: true,
		records: []repository.OutboxRecord{
			{ID: 1, Topic: "auth-events", Key: "u1", Payload: []byte(`{"event_type":"user.account_deleted"}`)},
			{ID: 2, Topic: "auth-events", Key: "u2", Payload: []byte(`{"event_type":"session.revoked"}`)},
			{ID: 3, Topic: "auth-events", Key: "u3", Payload: []byte(`{"event_type":"session.revoked"}`)},
		},
	}
	pub := &fakePub{failAt: 2}
	worker := NewPublisherWithRunner(noopRunner{}, repo, pub, zap.NewNop(), DefaultOptions())

	n, err := worker.RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected kafka error")
	}
	if n != 1 || !slices.Equal(repo.published, []int64{1}) || !slices.Equal(repo.failed, []int64{2}) {
		t.Fatalf("n=%d published=%v failed=%v", n, repo.published, repo.failed)
	}
}

func TestRunOnceSkipsWhenLockBusy(t *testing.T) {
	repo := &fakeOutboxRepo{
		locked:  false,
		records: []repository.OutboxRecord{{ID: 1, Topic: "auth-events", Key: "u1", Payload: []byte(`{}`)}},
	}
	pub := &fakePub{}
	worker := NewPublisherWithRunner(noopRunner{}, repo, pub, zap.NewNop(), DefaultOptions())

	n, err := worker.RunOnce(context.Background())
	if err != nil || n != 0 || len(pub.sent) != 0 {
		t.Fatalf("busy lock must skip: n=%d err=%v sent=%v", n, err, pub.sent)
	}
}
