package runner

import (
	"errors"
	"sync"
)

// A function that is intended to be run as a goroutine which takes a channel
// that tells it when to quit.
type Runnable func(<-chan struct{}) error

var ErrDead = errors.New("Runner is dead")

type Runner struct {
	funcs []Runnable // The set of functions to be run in separate goroutines.

	// Optional functions to be run when the Runner stops and starts.
	setup   func() error
	cleanup func() error

	quit    chan struct{}
	errChan chan error // Used to collect errors.

	//
	wg sync.WaitGroup

	// A lock that ensures that Start, Stop, and Running are never called
	// at the same time.
	mtx     sync.RWMutex
	running bool
	dead    bool // If true, then the object cannot be restarted.
}

// Running tells whether the Runner is running.
func (r *Runner) Running() bool {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.running
}

// Start turns on the Runner.
func (r *Runner) Start() error {
	// Wait in case the Runner is resetting.
	r.Wait()

	r.mtx.Lock()
	defer r.mtx.Unlock()

	if r.dead {
		return ErrDead
	}

	if r.setup != nil {
		if r.setup() != nil {
			r.dead = true
			return ErrDead
		}
	}

	r.running = true
	r.quit = make(chan struct{})

	// start all the goroutines.
	for _, f := range r.funcs {
		r.wg.Add(1)
		go func(f func(<-chan struct{}) error) {
			err := f(r.quit)
			if err != nil {
				r.errChan <- err
			}
			r.wg.Done()
		}(f)
	}

	return nil
}

// Stop stops the Runner. The Runner is fully shut down once Stop returns.
func (r *Runner) Stop() {
	r.mtx.Lock()
	if !r.running {
		return
	}

	// Close
	defer func() {
		r.running = false
		r.mtx.Unlock()
	}()

	doneChan := make(chan struct{})

	// Wait for all the funcs to close.
	close(r.quit)

	// Make sure all goroutines close before the function closes.s
	go func() {
		r.wg.Wait()
		close(doneChan)
	}()

	// Collect all the errors.
out:
	for {
		select {
		case <-r.errChan:
			r.dead = true
		case <-doneChan:
			break out
		}
	}

	// Check whether any goroutine has returned an error.
	if r.cleanup != nil {
		if r.cleanup() != nil {
			r.dead = true
			return
		}
	}
}

// Wait returns when the Runner has fully shut down.
func (r *Runner) Wait() {
	// Grab the lock in case the Runner is changing state
	r.mtx.Lock()
	r.mtx.Unlock()

	// Wait for the last wait group to stop.
	r.wg.Wait()

	// The wait group is done before the Stop function
	// is finished, so we grab the lock again to ensure that Stop has completed.
	r.mtx.Lock()
	r.mtx.Unlock()
}

// New returns a new Runner object.
func New(funcs []Runnable, params ...func() error) *Runner {
	r := &Runner{
		funcs:   funcs,
		errChan: make(chan error),
	}

	for i, p := range params {
		switch i {
		case 0:
			r.setup = p
		case 1:
			r.cleanup = p
		default:
			break
		}
	}

	return r
}
