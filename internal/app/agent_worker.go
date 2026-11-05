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
	if a.agentScheduler == nil && a.domains.Budget == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	a.workerCancel, a.workerDone = cancel, done
	go func() {
		defer close(done)
		if a.domains.Budget != nil {
			recovered := make(chan struct{})
			go func() { defer close(recovered); a.recoverLLMReservations(ctx) }()
			defer func() { cancel(); <-recovered }()
		}
		workerID := uuid.NewString()
		backoff := time.Second
		for ctx.Err() == nil {
			var claimed bool
			var err error
			if a.agentScheduler != nil {
				claimed, err = a.agentScheduler.ClaimAndExecute(ctx, workerID)
			}
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

// Reservation recovery has its own bounded loop so a slow model request
// cannot delay crash settlement or permit resending an unknown dispatch.
func (a *App) recoverLLMReservations(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		cycle, cancel := context.WithTimeout(ctx, 10*time.Second)
		_, err := a.domains.Budget.RecoverReservations(cycle, 100)
		cancel()
		if err != nil && ctx.Err() == nil {
			log.Printf("llm reservation recovery failed: %T", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
