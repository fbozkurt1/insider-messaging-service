package worker

import (
	"context"
	"errors"
	"insider-message-service/internal/services"
	"sync"
	"time"
)

type JobManager struct {
	currentJob     *Job
	mu             sync.Mutex
	messageService services.MessageService
	isFirstRun     bool
	wg             *sync.WaitGroup
}

func NewJobManager(messageService services.MessageService, wg *sync.WaitGroup) *JobManager {
	return &JobManager{
		messageService: messageService,
		isFirstRun:     true,
		wg:             wg,
	}
}

const jobInterval = 2 * time.Second

// Starts a new job
func (m *JobManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentJob != nil {
		return errors.New("job is already running")
	}
	m.wg.Add(1)

	// Create and start the job
	m.currentJob = NewJob(jobInterval, m.messageService, m.isFirstRun)
	m.currentJob.Start(ctx, m.wg)

	// set the first run as complete
	if m.isFirstRun {
		m.isFirstRun = false
	}

	return nil
}

// Stops the active job
func (m *JobManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentJob == nil {
		return errors.New("actively running job not found")
	}

	m.currentJob.Stop()
	m.currentJob = nil
	return nil
}

// Checks if a job is currently running
func (m *JobManager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentJob != nil
}
