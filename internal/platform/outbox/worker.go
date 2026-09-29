package outbox

import (
	"context"
	"errors"
	"time"
)

// Run dispatches bounded local projection transactions until shutdown. Error
// observers receive only a constant code, never payloads or database causes.
func (s *Service) Run(ctx context.Context, onError func(string)) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_, e := s.DispatchOne(ctx)
		if e == nil {
			continue
		}
		if !errors.Is(e, ErrNoEvent) && onError != nil {
			onError("platform.internal_error")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
