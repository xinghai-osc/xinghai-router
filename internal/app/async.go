package app

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const (
	backgroundQueueSize  = 4096
	backgroundWorkers    = 4
	backgroundTaskBudget = 15 * time.Second
)

// backgroundWriter runs best-effort database writes outside the request path. Only work
// that may be lost without affecting correctness belongs here: channel health
// bookkeeping and api-key last-used stamps. Billing and request logs stay synchronous.
type backgroundWriter struct {
	tasks   chan func(context.Context)
	wg      sync.WaitGroup
	dropped atomic.Int64
	closed  atomic.Bool
	stop    context.CancelFunc
	once    sync.Once
}

func newBackgroundWriter() *backgroundWriter {
	ctx, cancel := context.WithCancel(context.Background())
	w := &backgroundWriter{tasks: make(chan func(context.Context), backgroundQueueSize), stop: cancel}
	for i := 0; i < backgroundWorkers; i++ {
		w.wg.Add(1)
		go w.run(ctx)
	}
	go w.reportDrops(ctx)
	return w
}

func (w *backgroundWriter) run(ctx context.Context) {
	defer w.wg.Done()
	for task := range w.tasks {
		taskCtx, cancel := context.WithTimeout(ctx, backgroundTaskBudget)
		task(taskCtx)
		cancel()
	}
}

func (w *backgroundWriter) reportDrops(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n := w.dropped.Swap(0); n > 0 {
				log.Printf("background writer dropped %d deferred writes (queue full)", n)
			}
		}
	}
}

// submit queues a task, dropping it when the queue is saturated so a slow database
// never adds latency to or blocks a gateway request.
func (w *backgroundWriter) submit(task func(context.Context)) {
	if w == nil || w.closed.Load() {
		return
	}
	// A request still in flight during shutdown can reach here after close() has closed
	// the channel; drop the task rather than panicking inside the handler.
	defer func() { _ = recover() }()
	select {
	case w.tasks <- task:
	default:
		w.dropped.Add(1)
	}
}

func (w *backgroundWriter) close() {
	if w == nil {
		return
	}
	w.once.Do(func() {
		w.closed.Store(true)
		close(w.tasks)
		w.wg.Wait()
		w.stop()
	})
}

// detach keeps the values of a request context (request id, tracing) while dropping its
// cancellation, so settlement and logging still complete when a client hangs up.
func detach(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), timeout)
}
