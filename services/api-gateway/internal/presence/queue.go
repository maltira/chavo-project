package presence

import "sync"

// queue — неограниченная FIFO-очередь задач для одной горутины. Канал не подходит: push вызывается под
// Service.mu, а задачи сами берут Service.mu, и заполненный канал привёл бы к взаимной блокировке.
type queue struct {
	mu     sync.Mutex
	items  []func()
	closed bool
	signal chan struct{}
}

func newQueue() *queue { return &queue{signal: make(chan struct{}, 1)} }

func (q *queue) push(f func()) {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.items = append(q.items, f)
	q.mu.Unlock()
	q.wake()
}

// close перестаёт принимать задачи; run выполнит оставшиеся и завершится.
func (q *queue) close() {
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()
	q.wake()
}

func (q *queue) wake() {
	select {
	case q.signal <- struct{}{}:
	default:
	}
}

func (q *queue) run() {
	for {
		q.mu.Lock()
		for len(q.items) == 0 {
			if q.closed {
				q.mu.Unlock()
				return
			}
			q.mu.Unlock()
			<-q.signal
			q.mu.Lock()
		}
		f := q.items[0]
		q.items[0] = nil
		q.items = q.items[1:]
		q.mu.Unlock()
		f()
	}
}
