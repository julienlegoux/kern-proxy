package oauth

// Ports: packages/ai/src/utils/oauth/device-code.ts. Generic device-code
// polling shared by every OAuth flow that uses it; the GitHub Copilot flow
// (copilot.go) is its first consumer (epic 12, issue 02). The Codex dual
// flow (issue 03) reuses it too.

import (
	"context"
	"errors"
	"time"
)

const (
	// deviceCodeMinimumIntervalMillis matches upstream's MINIMUM_INTERVAL_MS.
	deviceCodeMinimumIntervalMillis int64 = 1000
	// deviceCodeDefaultIntervalSeconds is RFC 8628 section 3.2's fallback
	// when the authorization server omits `interval`.
	deviceCodeDefaultIntervalSeconds = 5
	// deviceCodeSlowDownIncrementMillis is RFC 8628 section 3.5's required
	// back-off step when a `slow_down` response carries no explicit interval.
	deviceCodeSlowDownIncrementMillis int64 = 5000
)

// DeviceCodePollStatus discriminates one poll iteration's outcome.
type DeviceCodePollStatus string

const (
	DeviceCodePollPending  DeviceCodePollStatus = "pending"
	DeviceCodePollSlowDown DeviceCodePollStatus = "slow_down"
	DeviceCodePollFailed   DeviceCodePollStatus = "failed"
	DeviceCodePollComplete DeviceCodePollStatus = "complete"
)

// DeviceCodePollResult is the outcome of one call to a
// DeviceCodePollOptions.Poll function.
type DeviceCodePollResult[T any] struct {
	Status DeviceCodePollStatus

	// IntervalSeconds is the server-requested new minimum poll interval for
	// a "slow_down" status; 0 means the server didn't provide one.
	IntervalSeconds int

	// Message is the error text for a "failed" status.
	Message string

	// Value is the result for a "complete" status.
	Value T
}

// DeviceCodePollOptions configures PollDeviceCodeFlow.
type DeviceCodePollOptions[T any] struct {
	// IntervalSeconds is the authorization server's advertised poll
	// interval; <= 0 falls back to deviceCodeDefaultIntervalSeconds.
	IntervalSeconds int
	// ExpiresInSeconds bounds the overall poll; <= 0 means no deadline.
	ExpiresInSeconds int
	// WaitBeforeFirstPoll delays the first Poll call by one interval,
	// matching GitHub's device flow (the user needs time to enter the code).
	WaitBeforeFirstPoll bool
	// Poll performs one poll request.
	Poll func(ctx context.Context) (DeviceCodePollResult[T], error)
}

var (
	// ErrDeviceCodeCancelled is returned when ctx is done or cancelled while
	// PollDeviceCodeFlow is waiting or polling.
	ErrDeviceCodeCancelled = errors.New("oauth: login cancelled")

	errDeviceCodeTimeout = errors.New("oauth: device flow timed out")

	// errDeviceCodeSlowDownTimeout matches upstream's clock-drift hint, shown
	// only when at least one slow_down response was observed before timeout.
	errDeviceCodeSlowDownTimeout = errors.New("oauth: device flow timed out after one or more slow_down responses. " +
		"This is often caused by clock drift in WSL or VM environments. Please sync or restart the VM clock and try again.")
)

// deviceCodeSleep is a package-level var so tests can fast-forward a virtual
// clock instead of sleeping in real time, mirroring token.go's clock var. The
// real implementation respects ctx cancellation.
var deviceCodeSleep = func(ctx context.Context, millis int64) error {
	if millis <= 0 {
		return nil
	}
	timer := time.NewTimer(time.Duration(millis) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// PollDeviceCodeFlow polls opts.Poll on an interval until it reports
// "complete", "failed", ctx is done, or opts.ExpiresInSeconds elapses.
// Matches upstream's pollOAuthDeviceCodeFlow, including RFC 8628 section
// 3.5's slow_down back-off (server-provided interval when given, otherwise a
// 5-second increase).
func PollDeviceCodeFlow[T any](ctx context.Context, opts DeviceCodePollOptions[T]) (T, error) {
	var zero T

	hasDeadline := opts.ExpiresInSeconds > 0
	var deadline int64
	if hasDeadline {
		deadline = clock() + int64(opts.ExpiresInSeconds)*1000
	}

	intervalSeconds := opts.IntervalSeconds
	if intervalSeconds <= 0 {
		intervalSeconds = deviceCodeDefaultIntervalSeconds
	}
	intervalMillis := int64(intervalSeconds) * 1000
	if intervalMillis < deviceCodeMinimumIntervalMillis {
		intervalMillis = deviceCodeMinimumIntervalMillis
	}

	slowDownResponses := 0

	if opts.WaitBeforeFirstPoll {
		wait := intervalMillis
		if hasDeadline {
			remaining := deadline - clock()
			if remaining <= 0 {
				return zero, errDeviceCodeTimeout
			}
			if remaining < wait {
				wait = remaining
			}
		}
		if err := deviceCodeSleep(ctx, wait); err != nil {
			return zero, ErrDeviceCodeCancelled
		}
	}

	for !hasDeadline || clock() < deadline {
		select {
		case <-ctx.Done():
			return zero, ErrDeviceCodeCancelled
		default:
		}

		result, err := opts.Poll(ctx)
		if err != nil {
			return zero, err
		}

		switch result.Status {
		case DeviceCodePollComplete:
			return result.Value, nil
		case DeviceCodePollFailed:
			msg := result.Message
			if msg == "" {
				msg = "device flow failed"
			}
			return zero, errors.New(msg)
		case DeviceCodePollSlowDown:
			slowDownResponses++
			if result.IntervalSeconds > 0 {
				intervalMillis = int64(result.IntervalSeconds) * 1000
			} else {
				intervalMillis += deviceCodeSlowDownIncrementMillis
			}
			if intervalMillis < deviceCodeMinimumIntervalMillis {
				intervalMillis = deviceCodeMinimumIntervalMillis
			}
		case DeviceCodePollPending:
			// interval unchanged
		}

		wait := intervalMillis
		if hasDeadline {
			remaining := deadline - clock()
			if remaining <= 0 {
				break
			}
			if remaining < wait {
				wait = remaining
			}
		}

		if err := deviceCodeSleep(ctx, wait); err != nil {
			return zero, ErrDeviceCodeCancelled
		}
	}

	if slowDownResponses > 0 {
		return zero, errDeviceCodeSlowDownTimeout
	}
	return zero, errDeviceCodeTimeout
}
