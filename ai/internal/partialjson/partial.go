package partialjson

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf16"
)

// ParsePartial parses possibly-truncated JSON, returning the value parsed so
// far. It mirrors the behavior of the npm `partial-json` parser with all
// partial types allowed:
//
//   - a truncated string returns its decoded prefix
//   - a truncated number/literal returns its completed value when unambiguous
//   - a truncated array returns the complete elements plus a partial last
//     element when one can be parsed
//   - a truncated object returns complete key/value pairs plus a partial last
//     value; a dangling key with no value is dropped
//
// It returns an error only when nothing meaningful can be parsed.
func ParsePartial(s string) (any, error) {
	p := &partialParser{s: s}
	p.skipWS()
	if p.pos >= len(p.s) {
		return nil, errUnparsable
	}
	v, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	return v, nil
}

var errUnparsable = errors.New("partialjson: unparsable input")

// errIncompleteScalar signals a value whose prefix is too ambiguous to keep
// (e.g. a bare "-" or "t" that stops mid-literal at a nesting boundary where
// partial scalars aren't returned).
var errIncompleteScalar = errors.New("partialjson: incomplete scalar")

type partialParser struct {
	s   string
	pos int
}

func (p *partialParser) skipWS() {
	for p.pos < len(p.s) {
		switch p.s[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

func (p *partialParser) parseValue() (any, error) {
	p.skipWS()
	if p.pos >= len(p.s) {
		return nil, errIncompleteScalar
	}
	switch c := p.s[p.pos]; {
	case c == '{':
		return p.parseObject()
	case c == '[':
		return p.parseArray()
	case c == '"':
		return p.parseString()
	case c == 't' || c == 'f' || c == 'n':
		return p.parseLiteral()
	case c == '-' || (c >= '0' && c <= '9'):
		return p.parseNumber()
	default:
		return nil, errUnparsable
	}
}

func (p *partialParser) parseLiteral() (any, error) {
	rest := p.s[p.pos:]
	for lit, v := range map[string]any{"true": true, "false": false, "null": nil} {
		if strings.HasPrefix(rest, lit) {
			p.pos += len(lit)
			return v, nil
		}
		// Truncated literal at end of input: complete it.
		if strings.HasPrefix(lit, rest) {
			p.pos = len(p.s)
			return v, nil
		}
	}
	return nil, errUnparsable
}

func (p *partialParser) parseNumber() (any, error) {
	start := p.pos
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		if (c >= '0' && c <= '9') || c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E' {
			p.pos++
			continue
		}
		break
	}
	text := p.s[start:p.pos]
	truncated := p.pos >= len(p.s)
	v, err := parseNumberText(text)
	if err == nil {
		return v, nil
	}
	if truncated {
		// Trim trailing partial exponent/sign/dot until parseable.
		for len(text) > 0 {
			text = text[:len(text)-1]
			if v, err := parseNumberText(text); err == nil {
				return v, nil
			}
		}
		return nil, errIncompleteScalar
	}
	return nil, errUnparsable
}

func parseNumberText(text string) (any, error) {
	if text == "" {
		return nil, errUnparsable
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return nil, errUnparsable
	}
	return f, nil
}

// parseString decodes a JSON string; when the closing quote is missing, the
// decoded prefix is returned (dropping a trailing incomplete escape).
func (p *partialParser) parseString() (any, error) {
	// p.s[p.pos] == '"'
	p.pos++
	var b strings.Builder
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		if c == '"' {
			p.pos++
			return b.String(), nil
		}
		if c == '\\' {
			if p.pos+1 >= len(p.s) {
				// Trailing incomplete escape: drop it.
				p.pos = len(p.s)
				return b.String(), nil
			}
			esc := p.s[p.pos+1]
			switch esc {
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			case '/':
				b.WriteByte('/')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'u':
				if p.pos+6 > len(p.s) {
					// Incomplete unicode escape at end: drop it.
					p.pos = len(p.s)
					return b.String(), nil
				}
				code, err := strconv.ParseUint(p.s[p.pos+2:p.pos+6], 16, 32)
				if err != nil {
					return nil, errUnparsable
				}
				r := rune(code)
				if utf16.IsSurrogate(r) && p.pos+12 <= len(p.s) && p.s[p.pos+6] == '\\' && p.s[p.pos+7] == 'u' {
					code2, err2 := strconv.ParseUint(p.s[p.pos+8:p.pos+12], 16, 32)
					if err2 == nil {
						combined := utf16.DecodeRune(r, rune(code2))
						if combined != 0xFFFD {
							b.WriteRune(combined)
							p.pos += 12
							continue
						}
					}
				}
				b.WriteRune(r)
				p.pos += 6
				continue
			default:
				return nil, errUnparsable
			}
			p.pos += 2
			continue
		}
		b.WriteByte(c)
		p.pos++
	}
	// Unterminated string: return the prefix.
	return b.String(), nil
}

func (p *partialParser) parseArray() (any, error) {
	// p.s[p.pos] == '['
	p.pos++
	out := []any{}
	for {
		p.skipWS()
		if p.pos >= len(p.s) {
			return out, nil
		}
		if p.s[p.pos] == ']' {
			p.pos++
			return out, nil
		}
		v, err := p.parseValue()
		if err != nil {
			if errors.Is(err, errIncompleteScalar) {
				return out, nil
			}
			return nil, err
		}
		out = append(out, v)
		p.skipWS()
		if p.pos >= len(p.s) {
			return out, nil
		}
		switch p.s[p.pos] {
		case ',':
			p.pos++
		case ']':
			p.pos++
			return out, nil
		default:
			return nil, errUnparsable
		}
	}
}

func (p *partialParser) parseObject() (any, error) {
	// p.s[p.pos] == '{'
	p.pos++
	out := map[string]any{}
	for {
		p.skipWS()
		if p.pos >= len(p.s) {
			return out, nil
		}
		if p.s[p.pos] == '}' {
			p.pos++
			return out, nil
		}
		if p.s[p.pos] != '"' {
			return nil, errUnparsable
		}
		keyStart := p.pos
		key, err := p.parseString()
		if err != nil {
			return nil, err
		}
		// A key whose closing quote was truncated has no value: drop it.
		if p.pos >= len(p.s) && !strings.HasSuffix(p.s[keyStart:], `"`) {
			return out, nil
		}
		if p.pos >= len(p.s) || func() bool { p.skipWS(); return p.pos >= len(p.s) }() {
			return out, nil
		}
		if p.s[p.pos] != ':' {
			return nil, errUnparsable
		}
		p.pos++
		p.skipWS()
		if p.pos >= len(p.s) {
			// Key with no value yet: drop it.
			return out, nil
		}
		v, err := p.parseValue()
		if err != nil {
			if errors.Is(err, errIncompleteScalar) {
				return out, nil
			}
			return nil, err
		}
		out[key.(string)] = v
		p.skipWS()
		if p.pos >= len(p.s) {
			return out, nil
		}
		switch p.s[p.pos] {
		case ',':
			p.pos++
		case '}':
			p.pos++
			return out, nil
		default:
			return nil, errUnparsable
		}
	}
}
