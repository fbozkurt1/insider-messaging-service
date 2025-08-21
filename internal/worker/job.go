package worker

import (
	"context"
	"insider-message-service/internal/services"
	"log"
	"sync"
	"time"
)

type Job struct {
	ticker         *time.Ticker
	quit           chan struct{}
	messageService services.MessageService
	isRunning      bool
	isFirstRun     bool
	mu             sync.Mutex
}

func NewJob(interval time.Duration, messageService services.MessageService, isFirstRun bool) *Job {
	return &Job{
		ticker:         time.NewTicker(interval),
		quit:           make(chan struct{}),
		messageService: messageService,
		isRunning:      false,
		isFirstRun:     isFirstRun,
	}
}

func (j *Job) Start(ctx context.Context, wg *sync.WaitGroup) {
	log.Println("Sending messages job started!")
	go func() {
		// if this is the first run, send all pending messages
		if j.isFirstRun {
			j.sendMessages(ctx)
		}

		for {
			select {
			case <-j.ticker.C:
				j.sendMessages(ctx)
			case <-j.quit:
				log.Println("Stopping sending messages job by toggle")
				j.ticker.Stop()
				return
			case <-ctx.Done():
				j.ticker.Stop()
				log.Println("Application shutdown signal received, stopping sending messages job")
				wg.Done()
				return
			}
		}
	}()
}

func (j *Job) Stop() {
	close(j.quit)
	log.Println("Sending messages job stopped!")
}

func (j *Job) sendMessages(ctx context.Context) {
	j.mu.Lock()
	if j.isRunning {
		log.Println("Job is already running, skipping this run")
		j.mu.Unlock()
		return
	}

	j.isRunning = true
	j.mu.Unlock()

	defer func() {
		j.mu.Lock()
		j.isRunning = false
		j.mu.Unlock()
	}()

	var err error
	if j.isFirstRun {
		log.Println("This is first run so sending all pending messages")
		err = j.messageService.SendAllPendingMessages(ctx)

		j.mu.Lock()
		j.isFirstRun = false
		j.mu.Unlock()
	} else {
		var numOfMessagesToSend int32 = 2
		log.Printf("This is not first run. So will try to send %d pending messages\n", numOfMessagesToSend)
		processedInBatch, processableCount, err := j.messageService.SendPendingMessagesWithLimit(ctx, numOfMessagesToSend)
		if err == nil {
			log.Printf("Processed %d out of %d messages\n", processedInBatch, processableCount)
		}
	}

	if err != nil {
		log.Printf("Unexpected error while sending pending messages: %v\n", err)
		return
	}
}
