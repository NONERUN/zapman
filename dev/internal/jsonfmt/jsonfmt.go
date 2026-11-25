// Package jsonfmt formats the JSON files of the catalog: an object or an
// array that fits in a line stays on one line, and the key order does not
// change.
//
// The dev tools use this package. The product does not use it.
package jsonfmt

import (
	"bytes"
	"encoding/json"
	"strings"
)

// lineWidth is the width up to which an object or an array stays on one
// line.
const lineWidth = 120

// Marshal is json.Marshal without the HTML escapes.
func Marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// node is a JSON value that keeps the key order.
type node struct {
	kind byte // '{', '[' or 0 for a scalar
	keys []string
	vals []*node
	raw  string
}

// Pretty formats a JSON text. An object or an array that fits in lineWidth
// stays on one line. The key order does not change.
func Pretty(text string) []byte {
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	n, err := parseNode(dec)
	if err != nil {
		panic(err)
	}
	return []byte(format(n, 0, 0, true) + "\n")
}

func parseNode(dec *json.Decoder) (*node, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		n := &node{kind: byte(t)}
		for dec.More() {
			if n.kind == '{' {
				k, err := dec.Token()
				if err != nil {
					return nil, err
				}
				n.keys = append(n.keys, k.(string))
			}
			v, err := parseNode(dec)
			if err != nil {
				return nil, err
			}
			n.vals = append(n.vals, v)
		}
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
		return n, nil
	case json.Number:
		return &node{raw: t.String()}, nil
	case nil:
		return &node{raw: "null"}, nil
	default:
		b, err := Marshal(t)
		if err != nil {
			return nil, err
		}
		return &node{raw: string(b)}, nil
	}
}

func compact(n *node) string {
	switch n.kind {
	case '{':
		parts := make([]string, len(n.vals))
		for i := range n.vals {
			k, _ := Marshal(n.keys[i])
			parts[i] = string(k) + ": " + compact(n.vals[i])
		}
		return "{ " + strings.Join(parts, ", ") + " }"
	case '[':
		parts := make([]string, len(n.vals))
		for i := range n.vals {
			parts[i] = compact(n.vals[i])
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return n.raw
}

// format gives the text of n at the indent. prefix is the length of the
// text before n on its line. The root is always on more lines.
func format(n *node, indent, prefix int, root bool) string {
	if n.kind == 0 || len(n.vals) == 0 {
		if n.kind == '{' {
			return "{}"
		}
		if n.kind == '[' {
			return "[]"
		}
		return n.raw
	}
	if c := compact(n); !root && indent+prefix+len(c) <= lineWidth {
		return c
	}
	pad := strings.Repeat("  ", indent/2+1)
	var b strings.Builder
	b.WriteByte(n.kind)
	b.WriteByte('\n')
	for i, v := range n.vals {
		b.WriteString(pad)
		p := 0
		if n.kind == '{' {
			k, _ := Marshal(n.keys[i])
			b.Write(k)
			b.WriteString(": ")
			p = len(k) + 2
		}
		b.WriteString(format(v, indent+2, p, false))
		if i < len(n.vals)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString(strings.Repeat("  ", indent/2))
	if n.kind == '{' {
		b.WriteByte('}')
	} else {
		b.WriteByte(']')
	}
	return b.String()
}
