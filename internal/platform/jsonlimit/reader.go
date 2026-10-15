// Package jsonlimit bounds JSON allocation hazards before typed decoding.
package jsonlimit

import (
	"errors"
	"io"
)

// ErrLimit reports a document exceeding its structural allocation budget.
var ErrLimit = errors.New("JSON allocation limit exceeded")

// Limits are independent; zero means unlimited except Depth (default 64).
type Limits struct{ Depth, ArrayItems, Nodes, StringBytes int }

type frame struct {
	array, expect bool
	items         int
}

// Reader checks structure without building an object tree. A json.Decoder must
// still validate syntax. Strings count encoded bytes, conservatively.
type Reader struct {
	source                    io.Reader
	limits                    Limits
	stack                     [128]frame
	depth, nodes, stringBytes int
	quoted, escaped, literal  bool
}

// NewReader constructs a bounded streaming lexical guard.
func NewReader(source io.Reader, limits Limits) *Reader {
	if limits.Depth <= 0 || limits.Depth > 128 {
		limits.Depth = 64
	}
	return &Reader{source: source, limits: limits}
}

func (r *Reader) begin() error {
	r.nodes++
	if r.limits.Nodes > 0 && r.nodes > r.limits.Nodes {
		return ErrLimit
	}
	if r.depth > 0 {
		f := &r.stack[r.depth-1]
		if f.array && f.expect {
			f.items++
			f.expect = false
			if r.limits.ArrayItems > 0 && f.items > r.limits.ArrayItems {
				return ErrLimit
			}
		}
	}
	return nil
}

func (r *Reader) inspect(data []byte) error {
	for _, c := range data {
		if r.quoted {
			if c == '"' && !r.escaped {
				r.quoted = false
				continue
			}
			r.stringBytes++
			if r.limits.StringBytes > 0 && r.stringBytes > r.limits.StringBytes {
				return ErrLimit
			}
			if r.escaped {
				r.escaped = false
			} else if c == '\\' {
				r.escaped = true
			}
			continue
		}
		switch c {
		case ' ', '\r', '\n', '\t', ':':
			r.literal = false
		case ',':
			r.literal = false
			if r.depth > 0 && r.stack[r.depth-1].array {
				r.stack[r.depth-1].expect = true
			}
		case '}', ']':
			r.literal = false
			if r.depth > 0 {
				r.depth--
			}
		case '{', '[':
			r.literal = false
			if err := r.begin(); err != nil {
				return err
			}
			if r.depth >= r.limits.Depth {
				return ErrLimit
			}
			r.stack[r.depth] = frame{array: c == '[', expect: true}
			r.depth++
		case '"':
			if err := r.begin(); err != nil {
				return err
			}
			r.quoted = true
			r.stringBytes = 0
			r.literal = false
		default:
			if !r.literal {
				if err := r.begin(); err != nil {
					return err
				}
				r.literal = true
			}
		}
	}
	return nil
}

func (r *Reader) Read(data []byte) (int, error) {
	n, err := r.source.Read(data)
	if limitErr := r.inspect(data[:n]); limitErr != nil {
		return 0, limitErr
	}
	return n, err
}
