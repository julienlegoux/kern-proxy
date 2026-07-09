package oauth

// Ports: packages/ai/test/oauth-device-code.test.ts. Upstream drives these
// cases with vitest's fake timers, asserting exact wall-clock poll
// timestamps as `vi.advanceTimersByTimeAsync` steps through time. Go has no
// equivalent of fake timers, so these tests instead override the package
// `clock` and `deviceCodeSleep` vars with an in-memory virtual clock that
// jumps forward by exactly the requested wait on every simulated sleep --
// same poll-timestamp assertions, no real waiting.

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

// withVirtualDeviceCodeClock overrides clock/deviceCodeSleep so the polling
// loop advances an in-memory timeline instead of sleeping in real time.
func withVirtualDeviceCodeClock(t *testing.T, start int64) {
	t.Helper()
	virtualNow := start
	originalClock := clock
	originalSleep := deviceCodeSleep
	clock = func() int64 { return virtualNow }
	deviceCodeSleep = func(ctx context.Context, millis int64) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if millis > 0 {
			virtualNow += millis
		}
		return nil
	}
	t.Cleanup(func() {
		clock = originalClock
		deviceCodeSleep = originalSleep
	})
}

func TestPollDeviceCodeFlow_PollsImmediatelyAndReturnsCompletedValue(t *testing.T) {
	withVirtualDeviceCodeClock(t, 0)

	var pollTimes []int64
	calls := 0
	result, err := PollDeviceCodeFlow(context.Background(), DeviceCodePollOptions[string]{
		IntervalSeconds:  2,
		ExpiresInSeconds: 30,
		Poll: func(context.Context) (DeviceCodePollResult[string], error) {
			pollTimes = append(pollTimes, clock())
			calls++
			if calls == 1 {
				return DeviceCodePollResult[string]{Status: DeviceCodePollPending}, nil
			}
			return DeviceCodePollResult[string]{Status: DeviceCodePollComplete, Value: "token"}, nil
		},
	})
	if err != nil {
		t.Fatalf("PollDeviceCodeFlow() error = %v", err)
	}
	if result != "token" {
		t.Errorf("result = %q, want token", result)
	}
	if want := []int64{0, 2000}; !reflect.DeepEqual(pollTimes, want) {
		t.Errorf("pollTimes = %v, want %v", pollTimes, want)
	}
}

func TestPollDeviceCodeFlow_CanWaitBeforeFirstPoll(t *testing.T) {
	withVirtualDeviceCodeClock(t, 0)

	var pollTimes []int64
	result, err := PollDeviceCodeFlow(context.Background(), DeviceCodePollOptions[string]{
		IntervalSeconds:     2,
		ExpiresInSeconds:    30,
		WaitBeforeFirstPoll: true,
		Poll: func(context.Context) (DeviceCodePollResult[string], error) {
			pollTimes = append(pollTimes, clock())
			return DeviceCodePollResult[string]{Status: DeviceCodePollComplete, Value: "token"}, nil
		},
	})
	if err != nil {
		t.Fatalf("PollDeviceCodeFlow() error = %v", err)
	}
	if result != "token" {
		t.Errorf("result = %q, want token", result)
	}
	if want := []int64{2000}; !reflect.DeepEqual(pollTimes, want) {
		t.Errorf("pollTimes = %v, want %v", pollTimes, want)
	}
}

func TestPollDeviceCodeFlow_SlowDownWithoutServerInterval(t *testing.T) {
	withVirtualDeviceCodeClock(t, 0)

	var pollTimes []int64
	calls := 0
	result, err := PollDeviceCodeFlow(context.Background(), DeviceCodePollOptions[string]{
		IntervalSeconds:  2,
		ExpiresInSeconds: 900,
		Poll: func(context.Context) (DeviceCodePollResult[string], error) {
			pollTimes = append(pollTimes, clock())
			calls++
			if calls == 1 {
				return DeviceCodePollResult[string]{Status: DeviceCodePollSlowDown}, nil
			}
			return DeviceCodePollResult[string]{Status: DeviceCodePollComplete, Value: "token"}, nil
		},
	})
	if err != nil {
		t.Fatalf("PollDeviceCodeFlow() error = %v", err)
	}
	if result != "token" {
		t.Errorf("result = %q, want token", result)
	}
	// No server interval on slow_down: back off by +5s over the 2s base.
	if want := []int64{0, 7000}; !reflect.DeepEqual(pollTimes, want) {
		t.Errorf("pollTimes = %v, want %v", pollTimes, want)
	}
}

func TestPollDeviceCodeFlow_HonorsServerProvidedSlowDownInterval(t *testing.T) {
	withVirtualDeviceCodeClock(t, 0)

	var pollTimes []int64
	calls := 0
	result, err := PollDeviceCodeFlow(context.Background(), DeviceCodePollOptions[string]{
		IntervalSeconds:  2,
		ExpiresInSeconds: 900,
		Poll: func(context.Context) (DeviceCodePollResult[string], error) {
			pollTimes = append(pollTimes, clock())
			calls++
			if calls == 1 {
				return DeviceCodePollResult[string]{Status: DeviceCodePollSlowDown, IntervalSeconds: 30}, nil
			}
			return DeviceCodePollResult[string]{Status: DeviceCodePollComplete, Value: "token"}, nil
		},
	})
	if err != nil {
		t.Fatalf("PollDeviceCodeFlow() error = %v", err)
	}
	if result != "token" {
		t.Errorf("result = %q, want token", result)
	}
	if want := []int64{0, 30000}; !reflect.DeepEqual(pollTimes, want) {
		t.Errorf("pollTimes = %v, want %v", pollTimes, want)
	}
}

func TestPollDeviceCodeFlow_CancelsInFlightWait(t *testing.T) {
	// Deliberately uses the real (non-virtual) clock/sleep: cancellation
	// short-circuits the timer immediately via ctx.Done(), so this
	// completes near-instantly despite a 5s interval -- no real wait.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pollCh := make(chan struct{}, 1)
	var pollCount int32

	resultCh := make(chan error, 1)
	go func() {
		_, err := PollDeviceCodeFlow(ctx, DeviceCodePollOptions[string]{
			IntervalSeconds:  5,
			ExpiresInSeconds: 30,
			Poll: func(context.Context) (DeviceCodePollResult[string], error) {
				if atomic.AddInt32(&pollCount, 1) == 1 {
					pollCh <- struct{}{}
				}
				return DeviceCodePollResult[string]{Status: DeviceCodePollPending}, nil
			},
		})
		resultCh <- err
	}()

	<-pollCh
	cancel()

	select {
	case err := <-resultCh:
		if !errors.Is(err, ErrDeviceCodeCancelled) {
			t.Fatalf("PollDeviceCodeFlow() error = %v, want ErrDeviceCodeCancelled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for PollDeviceCodeFlow to observe cancellation")
	}
}

func TestPollDeviceCodeFlow_TimesOutAfterSlowDown(t *testing.T) {
	withVirtualDeviceCodeClock(t, 0)

	_, err := PollDeviceCodeFlow(context.Background(), DeviceCodePollOptions[string]{
		IntervalSeconds:  5,
		ExpiresInSeconds: 20,
		Poll: func(context.Context) (DeviceCodePollResult[string], error) {
			return DeviceCodePollResult[string]{Status: DeviceCodePollSlowDown}, nil
		},
	})
	if err == nil {
		t.Fatal("PollDeviceCodeFlow() error = nil, want a slow_down timeout error")
	}
	if err.Error() != errDeviceCodeSlowDownTimeout.Error() {
		t.Errorf("error = %q, want %q", err.Error(), errDeviceCodeSlowDownTimeout.Error())
	}
}

func TestPollDeviceCodeFlow_TimesOutWithoutSlowDown(t *testing.T) {
	withVirtualDeviceCodeClock(t, 0)

	_, err := PollDeviceCodeFlow(context.Background(), DeviceCodePollOptions[string]{
		IntervalSeconds:  5,
		ExpiresInSeconds: 12,
		Poll: func(context.Context) (DeviceCodePollResult[string], error) {
			return DeviceCodePollResult[string]{Status: DeviceCodePollPending}, nil
		},
	})
	if err == nil {
		t.Fatal("PollDeviceCodeFlow() error = nil, want a timeout error")
	}
	if err.Error() != errDeviceCodeTimeout.Error() {
		t.Errorf("error = %q, want %q", err.Error(), errDeviceCodeTimeout.Error())
	}
}
