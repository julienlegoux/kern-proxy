// Package sse implements a hand-rolled Server-Sent Events reader matching the
// line-handling semantics of pi-ai's SSE parsing (CR, LF, and CRLF line
// terminators; field decoding; blank-line dispatch).
//
// Ports: packages/ai/src/api/anthropic-messages.ts (iterateSseMessages)
package sse

import (
	"bufio"
	"io"
	"strings"
)

// Event is one server-sent event.
type Event struct {
	// Event is the event name ("" when the server sent none).
	Event string
	// Data is the event payload; multiple data: lines are joined with "\n".
	Data string
	// ID is the last-event-id field, if any.
	ID string
	// Retry is the raw retry field, if any.
	Retry string
}

// Reader incrementally parses an SSE byte stream.
type Reader struct {
	r *bufio.Reader
	// pendingLF skips the \n of a \r\n pair split across reads.
	pendingLF bool
	line      strings.Builder

	event   Event
	hasData bool
	dataBuf strings.Builder
}

// NewReader wraps an io.Reader (typically a streaming HTTP response body).
func NewReader(r io.Reader) *Reader {
	return &Reader{r: bufio.NewReader(r)}
}

// Next returns the next complete event. It returns io.EOF at end of stream;
// an event still being accumulated at EOF is discarded, matching the
// browser/TS behavior of dispatching only on blank lines.
func (p *Reader) Next() (Event, error) {
	for {
		line, err := p.readLine()
		if err != nil {
			return Event{}, err
		}
		if line == "" {
			// Blank line: dispatch if the event carries anything.
			if p.hasData || p.event.Event != "" || p.event.ID != "" || p.event.Retry != "" {
				ev := p.event
				ev.Data = p.dataBuf.String()
				p.event = Event{}
				p.hasData = false
				p.dataBuf.Reset()
				return ev, nil
			}
			continue
		}
		p.consumeFieldLine(line)
	}
}

// readLine reads one line terminated by \r, \n, or \r\n.
func (p *Reader) readLine() (string, error) {
	p.line.Reset()
	for {
		b, err := p.r.ReadByte()
		if err != nil {
			if err == io.EOF && p.line.Len() > 0 {
				// Trailing line without terminator: browsers buffer it until a
				// terminator arrives; at EOF it is dropped with the partial
				// event. Surface EOF.
				return "", io.EOF
			}
			return "", err
		}
		if p.pendingLF {
			p.pendingLF = false
			if b == '\n' {
				continue // second half of \r\n
			}
		}
		switch b {
		case '\n':
			return p.line.String(), nil
		case '\r':
			p.pendingLF = true
			return p.line.String(), nil
		default:
			p.line.WriteByte(b)
		}
	}
}

// consumeFieldLine decodes one "field: value" line into the pending event.
func (p *Reader) consumeFieldLine(line string) {
	if strings.HasPrefix(line, ":") {
		return // comment
	}
	field, value, found := strings.Cut(line, ":")
	if !found {
		field, value = line, ""
	}
	value = strings.TrimPrefix(value, " ")
	switch field {
	case "event":
		p.event.Event = value
	case "data":
		if p.hasData {
			p.dataBuf.WriteString("\n")
		}
		p.dataBuf.WriteString(value)
		p.hasData = true
	case "id":
		p.event.ID = value
	case "retry":
		p.event.Retry = value
	}
}
