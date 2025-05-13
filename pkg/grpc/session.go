package grpc

import (
	"container/heap"
	"sync"
	"time"
)

// SessionJob represents a delayed job for session handling
type SessionJob struct {
	token    string
	claims   interface{} // Replace with your actual claims type
	runAt    time.Time
	canceled bool
	index    int // For heap implementation
}

// SessionJobHeap implements heap.Interface
type SessionJobHeap []*SessionJob

func (h SessionJobHeap) Len() int           { return len(h) }
func (h SessionJobHeap) Less(i, j int) bool { return h[i].runAt.Before(h[j].runAt) }
func (h SessionJobHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *SessionJobHeap) Push(x interface{}) {
	n := len(*h)
	job := x.(*SessionJob)
	job.index = n
	*h = append(*h, job)
}

func (h *SessionJobHeap) Pop() interface{} {
	old := *h
	n := len(old)
	job := old[n-1]
	old[n-1] = nil
	job.index = -1
	*h = old[0 : n-1]
	return job
}

// SessionScheduler manages delayed session receipt submissions
type SessionScheduler struct {
	jobs     SessionJobHeap
	jobMap   map[string]*SessionJob
	mu       sync.Mutex
	timer    *time.Timer
	trigger  chan struct{}
	shutdown chan struct{}
	submitFn func(token string, claims interface{}) // Replace with your actual function signature
}

// NewSessionScheduler creates a new scheduler
func NewSessionScheduler(submitFn func(token string, claims interface{})) *SessionScheduler {
	s := &SessionScheduler{
		jobs:     make(SessionJobHeap, 0),
		jobMap:   make(map[string]*SessionJob),
		trigger:  make(chan struct{}, 1),
		shutdown: make(chan struct{}),
		submitFn: submitFn,
	}

	go s.run()
	return s
}

// ScheduleSessionReceipt schedules a job to submit a session receipt after a delay
func (s *SessionScheduler) ScheduleSessionReceipt(token string, claims interface{}, delay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if a job already exists for this token
	if job, exists := s.jobMap[token]; exists {
		// Update existing job
		job.claims = claims
		job.runAt = time.Now().Add(delay)
		job.canceled = false
		heap.Fix(&s.jobs, job.index)
	} else {
		// Create new job
		job := &SessionJob{
			token:    token,
			claims:   claims,
			runAt:    time.Now().Add(delay),
			canceled: false,
		}
		heap.Push(&s.jobs, job)
		s.jobMap[token] = job
	}

	// Signal that queue was updated
	select {
	case s.trigger <- struct{}{}:
	default:
	}
}

// CancelSessionReceipt cancels a scheduled session receipt job
func (s *SessionScheduler) CancelSessionReceipt(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobMap[token]
	if exists {
		job.canceled = true
		delete(s.jobMap, token)
		return true
	}
	return false
}

// run is the main scheduler loop
func (s *SessionScheduler) run() {
	for {
		s.mu.Lock()
		now := time.Now()

		// Process expired jobs
		for s.jobs.Len() > 0 && !s.jobs[0].runAt.After(now) {
			job := heap.Pop(&s.jobs).(*SessionJob)
			if !job.canceled {
				// Submit the receipt in a separate goroutine to avoid blocking
				job := job // Create a new variable to avoid data race
				go s.submitFn(job.token, job.claims)
			}
			delete(s.jobMap, job.token)
		}

		// Calculate the next wait time
		var waitDuration time.Duration
		if s.jobs.Len() > 0 {
			waitDuration = time.Until(s.jobs[0].runAt)
			if waitDuration < 0 {
				waitDuration = 0
			}
		} else {
			waitDuration = 24 * time.Hour // Default long wait if no jobs
		}

		// Reset or create timer
		if s.timer == nil {
			s.timer = time.NewTimer(waitDuration)
		} else {
			if !s.timer.Stop() {
				select {
				case <-s.timer.C:
				default:
				}
			}
			s.timer.Reset(waitDuration)
		}
		s.mu.Unlock()

		select {
		case <-s.timer.C:
			// Timer expired, process jobs
		case <-s.trigger:
			// Queue was updated, recalculate wait time
		case <-s.shutdown:
			if s.timer != nil {
				s.timer.Stop()
			}
			return
		}
	}
}

// Stop stops the scheduler
func (s *SessionScheduler) Stop() {
	close(s.shutdown)
}
