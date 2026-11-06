package main

import (
	"errors"
	"testing"
	"time"
)

func TestUXActionRejectsRepeatedClicksUntilCompletion(t *testing.T) {
	updates := make(chan func(), 4)
	started := make(chan struct{})
	release := make(chan struct{})
	repeated := make(chan struct{}, 1)
	state := &State{Update: func(update func()) { updates <- update }}
	defer close(release)
	state.runAction(func() error {
		close(started)
		<-release
		return nil
	}, "已恢复下载")
	if !state.Busy {
		t.Error("the action must become busy before another click can be handled")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("the first operation did not start")
	}
	state.runAction(func() error {
		repeated <- struct{}{}
		return nil
	}, "重复操作")
	select {
	case <-repeated:
		t.Fatal("a repeated click started a second operation")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestUXActionClearsBusyAfterSuccessOrFailure(t *testing.T) {
	for _, operationError := range []error{nil, errors.New("下载恢复失败")} {
		t.Run(operationLabel(operationError), func(t *testing.T) {
			updates := make(chan func(), 1)
			state := &State{Update: func(update func()) { updates <- update }}
			state.runAction(func() error { return operationError }, "已恢复下载")
			select {
			case update := <-updates:
				update()
			case <-time.After(time.Second):
				t.Fatal("the action did not publish its result")
			}
			if state.Busy {
				t.Fatal("the completed action leaves controls disabled")
			}
			if operationError != nil {
				if state.SaveError != operationError.Error() {
					t.Fatalf("failure is not visible: %q", state.SaveError)
				}
				return
			}
			if state.Notice != "已恢复下载" || state.SaveError != "" {
				t.Fatalf("success feedback is incorrect: notice=%q error=%q", state.Notice, state.SaveError)
			}
		})
	}
}

func operationLabel(operationError error) string {
	if operationError != nil {
		return "failure"
	}
	return "success"
}
