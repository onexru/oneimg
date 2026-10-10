package services

import (
	"context"
	"time"
)

const workerIdleMin = 3 * time.Second
const workerIdleMax = 30 * time.Second

// runDurableWorker uses the existing single goroutine. Drain active work without
// sleeps; bounded batches check maintenance without starving a continuous queue.
// Wake signals immediately reset idle backoff; timed retries remain durable.
func runDurableWorker(ctx context.Context, wake <-chan struct{}, process func() bool, maintenance func(), interval time.Duration, nextDue ...func() time.Time) {
	runDurableWorkerWithWait(ctx, wake, process, maintenance, interval, func(ctx context.Context, wake <-chan struct{}, delay time.Duration) (bool, bool) {
		if len(nextDue) > 0 && nextDue[0] != nil {
			delay = workerRetryDelay(delay, nextDue[0](), time.Now())
		}
		return waitForWorker(ctx, wake, delay)
	})
}
func runDurableWorkerWithWait(ctx context.Context, wake <-chan struct{}, process func() bool, maintenance func(), interval time.Duration, wait func(context.Context, <-chan struct{}, time.Duration) (bool, bool)) {
	idle := workerIdleMin
	nextMaintenance := time.Time{}
	for ctx.Err() == nil {
		if maintenance != nil && !time.Now().Before(nextMaintenance) {
			maintenance()
			nextMaintenance = time.Now().Add(interval)
		}
		processed := false
		for i := 0; i < 4 && ctx.Err() == nil; i++ {
			if !process() {
				break
			}
			processed = true
		}
		if processed {
			idle = workerIdleMin
			continue
		}
		// One reusable waiting operation, no ticker goroutine/periodic active delay.
		woke, ok := wait(ctx, wake, idle)
		if !ok {
			return
		}
		if woke {
			idle = workerIdleMin
		} else {
			idle *= 2
			if idle > workerIdleMax {
				idle = workerIdleMax
			}
		}
	}
}
func waitForWorker(ctx context.Context, wake <-chan struct{}, delay time.Duration) (bool, bool) {
	if ctx.Err() != nil {
		return false, false
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false, false
	case <-wake:
		return true, true
	case <-timer.C:
		return false, true
	}
}
