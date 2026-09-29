package app

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
)

// startAgentWorker starts only after routes, dependencies, and runtime validation succeeded.
// The process owns one worker; its context is independent of HTTP request lifetimes.
func (a *App) startAgentWorker() {
	if a.agentScheduler == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	a.workerCancel, a.workerDone = cancel, done
	go func() {
		defer close(done)
		workerID := uuid.NewString()
		backoff := time.Second
		for ctx.Err() == nil {
			claimed, err := a.agentScheduler.ClaimAndExecute(ctx, workerID)
			if ctx.Err() != nil {
				return
			}
			wait := time.Second
			if err != nil {
				// Do not include raw database/upstream errors (which can carry SQL or secrets).
				log.Printf("agent worker cycle failed: %T", err)
				wait = backoff
				if backoff < 30*time.Second {
					backoff *= 2
				}
			} else {
				backoff = time.Second
				if claimed {
					wait = 250 * time.Millisecond
				}
			}
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
	if a.activateAgentTools != nil {
		a.activateAgentTools()
	}
}
