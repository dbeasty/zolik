// Package tsbundle reads the client's message bundle (a TypeScript object
// literal of string keys to string literals) into a Go map.
//
// It understands exactly the shape the bundles are written in — `'key': 'text',`
// with the text on the same line or the next, in single or double quotes,
// possibly several literals joined by `+`, and comments between entries — and
// refuses anything else rather than guessing at it.
package tsbundle

import (
	"fmt"
	"strconv"
	"strings"
)

// Parse returns every key of the first object literal in src.
func Parse(src string) (map[string]string, error) {
	start := strings.Index(src, "= {")
	if start < 0 {
		return nil, fmt.Errorf("tsbundle: no object literal")
	}
	p := &parser{src: src, i: start + len("= {")}
	out := map[string]string{}
	for {
		p.skip()
		if p.eof() {
			return nil, fmt.Errorf("tsbundle: unterminated object")
		}
		if p.peek() == '}' {
			return out, nil
		}
		key, err := p.literal()
		if err != nil {
			return nil, err
		}
		p.skip()
		if p.peek() != ':' {
			return nil, p.errorf("expected ':' after %q", key)
		}
		p.i++
		var val strings.Builder
		for {
			p.skip()
			s, err := p.literal()
			if err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			val.WriteString(s)
			p.skip()
			if p.peek() == '+' {
				p.i++
				continue
			}
			break
		}
		out[key] = val.String()
		p.skip()
		if p.peek() == ',' {
			p.i++
		}
	}
}

type parser struct {
	src string
	i   int
}

func (p *parser) eof() bool  { return p.i >= len(p.src) }
func (p *parser) peek() byte { return p.src[p.i] }

func (p *parser) errorf(format string, args ...any) error {
	line := strings.Count(p.src[:p.i], "\n") + 1
	return fmt.Errorf("tsbundle: line %d: %s", line, fmt.Sprintf(format, args...))
}

// skip passes whitespace and comments.
func (p *parser) skip() {
	for !p.eof() {
		switch {
		case strings.ContainsRune(" \t\r\n", rune(p.peek())):
			p.i++
		case strings.HasPrefix(p.src[p.i:], "//"):
			if j := strings.IndexByte(p.src[p.i:], '\n'); j >= 0 {
				p.i += j + 1
			} else {
				p.i = len(p.src)
			}
		case strings.HasPrefix(p.src[p.i:], "/*"):
			if j := strings.Index(p.src[p.i:], "*/"); j >= 0 {
				p.i += j + 2
			} else {
				p.i = len(p.src)
			}
		default:
			return
		}
	}
}

// literal reads one quoted string.
func (p *parser) literal() (string, error) {
	if p.eof() {
		return "", p.errorf("expected a string")
	}
	q := p.peek()
	if q != '\'' && q != '"' {
		return "", p.errorf("expected a string, found %q", q)
	}
	p.i++
	var b strings.Builder
	for !p.eof() {
		c := p.peek()
		switch {
		case c == q:
			p.i++
			return b.String(), nil
		case c == '\n':
			return "", p.errorf("newline in string")
		case c == '\\':
			if p.i+1 >= len(p.src) {
				return "", p.errorf("dangling escape")
			}
			e := p.src[p.i+1]
			p.i += 2
			switch e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'u':
				if p.i+4 > len(p.src) {
					return "", p.errorf("short \\u escape")
				}
				r, err := strconv.ParseUint(p.src[p.i:p.i+4], 16, 32)
				if err != nil {
					return "", p.errorf("bad \\u escape")
				}
				b.WriteRune(rune(r))
				p.i += 4
			default:
				b.WriteByte(e)
			}
		default:
			b.WriteByte(c)
			p.i++
		}
	}
	return "", p.errorf("unterminated string")
}
