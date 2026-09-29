package jobs

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Handler separates first dispatch from crash recovery. Recover must inspect
// durable domain checkpoints and execution receipts; it must never blindly call
// Execute again for a billed or side-effecting operation with unknown outcome.
type Handler interface {
	Execute(context.Context, Lease) error
	Recover(context.Context, Lease) error
}
type WorkerOptions struct {
	Domain       string
	Concurrency  int
	PollInterval time.Duration
	Handlers     map[string]Handler
	OnError      func(string)
}
type Worker struct {
	service *Service
	opts    WorkerOptions
}

func NewWorker(s *Service, o WorkerOptions) (*Worker, error) {
	if s == nil || !validDomain(o.Domain) || o.Concurrency < 1 || o.Concurrency > 64 || len(o.Handlers) == 0 {
		return nil, errors.New("invalid worker configuration")
	}
	if o.PollInterval == 0 {
		o.PollInterval = 250 * time.Millisecond
	}
	if o.PollInterval < 50*time.Millisecond || o.PollInterval > time.Minute {
		return nil, errors.New("invalid worker polling interval")
	}
	handlers := map[string]Handler{}
	for k, h := range o.Handlers {
		if !machine.MatchString(k) || h == nil {
			return nil, errors.New("invalid job handler")
		}
		handlers[k] = h
	}
	o.Handlers = handlers
	return &Worker{s, o}, nil
}

// Run stops claiming on context cancellation or SetDraining(true). It waits for
// owned handlers. Composition owns the 20 second drain deadline, then cancels ctx.
func (w *Worker) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	for i := 0; i < w.opts.Concurrency; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); w.loop(ctx, uuid.NewString()) }()
	}
	wg.Wait()
	return ctx.Err()
}
func (w *Worker) report(code string) {
	if w.opts.OnError != nil {
		w.opts.OnError(code)
	}
}
func (w *Worker) loop(ctx context.Context, worker string) {
	ticker := time.NewTicker(w.opts.PollInterval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil || w.service.draining.Load() {
			return
		}
		kinds := make([]string, 0, len(w.opts.Handlers))
		for kind := range w.opts.Handlers {
			kinds = append(kinds, kind)
		}
		lease, e := w.service.claim(ctx, worker, w.opts.Domain, kinds)
		if e == nil {
			w.execute(ctx, lease)
			continue
		}
		if errors.Is(e, ErrDraining) {
			return
		}
		if !errors.Is(e, ErrNoJob) {
			w.report("platform.internal_error")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (w *Worker) execute(parent context.Context, l Lease) {
	handler := w.opts.Handlers[l.job.JobKind]
	if handler == nil {
		w.report("platform.internal_error")
		return
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(HeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				beat, stop := context.WithTimeout(ctx, 5*time.Second)
				e := w.service.Heartbeat(beat, l)
				stop()
				if e != nil {
					cancel()
					w.report("platform.internal_error")
					return
				}
			}
		}
	}()
	var err error
	func() {
		defer func() {
			if recover() != nil {
				err = errors.New("job handler interrupted")
			}
		}()
		if l.recovered {
			err = handler.Recover(ctx, l)
		} else {
			err = handler.Execute(ctx, l)
		}
	}()
	close(done)
	cancel()
	<-stopped
	// A handler error is not permission to repeat an external effect. The durable
	// lease expires, and the next claimant is forced through Handler.Recover.
	if err != nil && parent.Err() == nil {
		w.report("platform.internal_error")
	}
}
