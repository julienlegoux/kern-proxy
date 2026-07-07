package sse

import (
	"io"
	"strings"
	"testing"
)

func readAll(t *testing.T, input string) []Event {
	t.Helper()
	r := NewReader(strings.NewReader(input))
	var events []Event
	for {
		ev, err := r.Next()
		if err == io.EOF {
			return events
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		events = append(events, ev)
	}
}

func TestBasicEvents(t *testing.T) {
	events := readAll(t, "event: message_start\ndata: {\"a\":1}\n\nevent: message_stop\ndata: {}\n\n")
	if len(events) != 2 {
		t.Fatalf("got %d events", len(events))
	}
	if events[0].Event != "message_start" || events[0].Data != `{"a":1}` {
		t.Errorf("event 0 = %+v", events[0])
	}
	if events[1].Event != "message_stop" {
		t.Errorf("event 1 = %+v", events[1])
	}
}

func TestLineTerminators(t *testing.T) {
	// CR, LF, and CRLF must all terminate lines.
	for name, input := range map[string]string{
		"lf":    "data: a\n\n",
		"crlf":  "data: a\r\n\r\n",
		"cr":    "data: a\r\r",
		"mixed": "event: e\rdata: a\r\n\n",
	} {
		t.Run(name, func(t *testing.T) {
			events := readAll(t, input)
			if len(events) != 1 || events[0].Data != "a" {
				t.Errorf("%s: events = %+v", name, events)
			}
		})
	}
}

func TestMultiLineData(t *testing.T) {
	events := readAll(t, "data: line1\ndata: line2\n\n")
	if len(events) != 1 || events[0].Data != "line1\nline2" {
		t.Errorf("events = %+v", events)
	}
}

func TestCommentsIgnored(t *testing.T) {
	events := readAll(t, ": keepalive\n\ndata: x\n\n")
	if len(events) != 1 || events[0].Data != "x" {
		t.Errorf("events = %+v", events)
	}
}

func TestFieldWithoutSpace(t *testing.T) {
	events := readAll(t, "data:tight\n\n")
	if len(events) != 1 || events[0].Data != "tight" {
		t.Errorf("events = %+v", events)
	}
}

func TestIncompleteEventAtEOFDiscarded(t *testing.T) {
	// No blank line: the trailing event never dispatches.
	events := readAll(t, "event: message_start\ndata: {\"a\":1}\n")
	if len(events) != 0 {
		t.Errorf("expected trailing undelivered event to be discarded, got %+v", events)
	}
}

func TestEmptyDataLine(t *testing.T) {
	events := readAll(t, "data:\n\n")
	if len(events) != 1 || events[0].Data != "" {
		t.Errorf("events = %+v", events)
	}
}

func TestIDAndRetryFields(t *testing.T) {
	events := readAll(t, "id: 42\nretry: 1000\ndata: x\n\n")
	if len(events) != 1 || events[0].ID != "42" || events[0].Retry != "1000" {
		t.Errorf("events = %+v", events)
	}
}
