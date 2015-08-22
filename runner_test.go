package runner_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DanielKrawisz/runner"
)

// A runnable test function.
var goFunc = func(i int, returnErr bool) runner.Runnable {
	return runner.Runnable(func(quit <-chan struct{}) error {

	out:
		for {
			select {
			case <-quit:
				fmt.Println("About to break out of function", i)
				break out
			}
		}

		if returnErr {
			return errors.New("This is an error.")
		}

		return nil
	})
}

// TestRunner tests starting and stoping the runner twice without errors.
func TestRunner(t *testing.T) {
	funcs := []runner.Runnable{goFunc(0, false), goFunc(1, false), goFunc(2, false)}
	run := runner.New(funcs)

	err := run.Start()
	if err != nil {
		t.Error("Failed to start: ", err)
	}

	time.Sleep(time.Second)

	run.Stop()

	err = run.Start()
	if err != nil {
		t.Error("Failed to start a second time: ", err)
	}
	time.Sleep(time.Second)

	run.Stop()
}

// TestRunnerErr tests error conditions and ensures that the runner dies.
func TestRunnerErr(t *testing.T) {
	funcs := []runner.Runnable{goFunc(0, true), goFunc(1, true), goFunc(2, false)}
	run := runner.New(funcs)

	err := run.Start()
	if err != nil {
		t.Error("Failed to start: ", err)
	}

	time.Sleep(time.Second)

	run.Stop()

	err = run.Start()
	if err == nil {
		t.Error("Should fail to start.")
	}
	time.Sleep(time.Second)

	run.Stop()
}

// TestRunnerWait tests whether the Wait function stops after Stop is done and
// doesn't cause a deadlock or race condition.
func TestRunnerWait(t *testing.T) {
	funcs := []runner.Runnable{goFunc(0, false), goFunc(1, false), goFunc(2, false)}
	run := runner.New(funcs)

	err := run.Start()
	if err != nil {
		t.Error("Failed to start: ", err)
	}

	startChan := make(chan struct{})
	doneWaitChan := make(chan struct{})
	doneStopChan := make(chan struct{})

	go func() {
		<-startChan
		run.Wait()
		doneWaitChan <- struct{}{}
	}()

	// This will cause Wait to be called.
	startChan <- struct{}{}

	time.Sleep(time.Second)

	go func() {
		<-startChan
		run.Stop()
		doneStopChan <- struct{}{}
	}()

	// This will cause Stop to be called.
	close(startChan)

	select {
	case <-doneWaitChan:
		t.Error("Wait ended before stop.")
	case <-doneStopChan:
	}

	<-doneWaitChan
}
