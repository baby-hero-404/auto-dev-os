package orchestrator

import (
	"context"
	"errors"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/observability"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"gorm.io/gorm"
)

func (o *Orchestrator) StartWorker(ctx context.Context, interval time.Duration, concurrency int) {
	if interval <= 0 {
		interval = 20 * time.Second
	}
	if concurrency <= 0 {
		concurrency = 1
	}

	sem := make(chan struct{}, concurrency)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		claimed := false
		for {
			select {
			case sem <- struct{}{}:
			default:
				goto wait
			}

			job, err := o.workflows.ClaimNext(ctx)
			if err != nil {
				<-sem
				if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, repository.ErrNotFound) {
					break
				}
				observability.Error(ctx, "claim workflow job failed", "error", err)
				break
			}
			claimed = true
			jobCtx, cancel := context.WithCancel(ctx)
			o.jobCancels.Store(job.ID, cancel)
			o.wg.Add(1)
			go func(jobID string, jCtx context.Context, cFunc context.CancelFunc) {
				defer o.wg.Done()
				defer func() {
					<-sem
					o.jobCancels.Delete(jobID)
					cFunc()
				}()
				o.run(jCtx, jobID)
			}(job.ID, jobCtx, cancel)
		}

	wait:
		if claimed {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-o.wakeChan:
		case <-ticker.C:
			_ = o.workflows.ResetStuckJobs(ctx)
		}
	}
}

func (o *Orchestrator) Wait() {
	o.wg.Wait()
}
