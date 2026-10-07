package events

import (
	"container/list"
	"sync"
	"time"
)

// dedup помнит обработанные event_id: доставка из Kafka at-least-once, повтор после ребаланса не должен дойти до клиента
// дважды. Записи добавляются в порядке времени, поэтому устаревшие и лишние вытесняются с начала списка.
type dedup struct {
	ttl  time.Duration
	size int

	mu    sync.Mutex
	order *list.List
	items map[string]*list.Element
}

type seenEntry struct {
	id string
	at time.Time
}

func newDedup(ttl time.Duration, size int) *dedup {
	return &dedup{ttl: ttl, size: size, order: list.New(), items: map[string]*list.Element{}}
}

func (d *dedup) has(id string, now time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.evict(now)
	_, ok := d.items[id]
	return ok
}

func (d *dedup) add(id string, now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.items[id]; ok {
		return
	}
	d.items[id] = d.order.PushBack(seenEntry{id: id, at: now})
	d.evict(now)
}

func (d *dedup) evict(now time.Time) {
	for e := d.order.Front(); e != nil; e = d.order.Front() {
		s := e.Value.(seenEntry)
		if len(d.items) <= d.size && now.Sub(s.at) < d.ttl {
			return
		}
		d.order.Remove(e)
		delete(d.items, s.id)
	}
}
