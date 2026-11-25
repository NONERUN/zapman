package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

//go:generate go run ../../dev/schemagen -out ../../strategies/schema

// SchemaDir is the folder of the JSON Schema files, in the strategies
// folder.
const SchemaDir = "schema"

// namePattern is the JSON Schema pattern of a name (see isName).
const namePattern = "^[a-z0-9]+(-[a-z0-9]+)*$"

// schemaBase is the $id prefix of the schema files.
const schemaBase = "https://github.com/NONERUN/zapman/strategies/schema/"

// Schemas gives the JSON Schema files of the v2 format, by file name. The
// schemas come from the Go types, the doc and schema struct tags, and the
// name tables of the validation.
func Schemas() map[string][]byte {
	g := &schemaGen{defs: map[string]*jsonObj{}}
	out := map[string][]byte{}
	put := func(file, title string, body *jsonObj) {
		root := &jsonObj{}
		root.set("$schema", "https://json-schema.org/draft/2020-12/schema")
		root.set("$id", schemaBase+file)
		root.set("title", title)
		root.kv = append(root.kv, body.kv...)
		if len(g.used) > 0 {
			defs := &jsonObj{}
			for _, n := range g.sortedUsed() {
				defs.set(n, g.defs[n])
			}
			root.set("$defs", defs)
		}
		b, err := marshalJSON(root)
		if err != nil {
			panic(err)
		}
		var ind bytes.Buffer
		if err := json.Indent(&ind, b, "", "  "); err != nil {
			panic(err)
		}
		out[file] = append(ind.Bytes(), '\n')
	}

	g.reset()
	put("catalog.json", "zapman catalog", g.object(reflect.TypeOf(CatalogInfo{})))
	g.reset()
	st := g.object(reflect.TypeOf(StrategyFile{}))
	// A flowseal/ strategy has upstream and rev. A zapman/ strategy has
	// version. The schema cannot see the folder: it accepts one of the two.
	flowseal, zapman := &jsonObj{}, &jsonObj{}
	flowseal.set("required", []string{"upstream", "rev"})
	zapman.set("required", []string{"version"})
	st.set("oneOf", []any{flowseal, zapman})
	put("strategy.json", "zapman strategy (spec 2)", st)
	g.reset()
	put("game.json", "zapman game module", g.object(reflect.TypeOf(GameFile{})))
	g.reset()
	put("lists.json", "zapman list bundles", g.named(reflect.TypeOf(Bundle{})))
	g.reset()
	put("targets.json", "zapman targets", g.named(reflect.TypeOf(Target{})))
	g.reset()
	put("actions.json", "zapman actions", g.named(reflect.TypeOf(Action{})))
	return out
}

// schemaPatterns are the patterns for the pattern struct tag.
var schemaPatterns = map[string]string{
	"name": namePattern,
	"port": `^(\$gameFilter|[0-9]+(-[0-9]+)?)$`,
	"tag":  tagPattern,
	// active is an active fake name (see isActiveName).
	"active": "^[A-Z0-9]+(_[A-Z0-9]+)*$",
	"catref": "^cat:[^ ]+$",
	// relpath is a file path from the catalog root, with /.
	"relpath": `^[^/\\]+(/[^/\\]+)*$`,
	"sha256":  "^[0-9a-f]{64}$",
}

// schemaEnums are the value lists for the enum and items struct tags.
func schemaEnums(name string) []string {
	var m map[string]bool
	switch name {
	case "desync":
		m = map[string]bool{}
		for k := range v01Attacks {
			m[k] = true
		}
	case "fooling":
		m = v01Fooling
	case "l7":
		m = v01L7
	case "updater":
		m = gameUpdaters
	default:
		panic("unknown schema enum " + name)
	}
	return sortedKeys(m)
}

type schemaGen struct {
	defs map[string]*jsonObj
	used map[string]bool
}

func (g *schemaGen) reset() { g.used = map[string]bool{} }

func (g *schemaGen) sortedUsed() []string {
	var out []string
	for n := range g.used {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// named gives the schema of a file that maps names to items of type t.
func (g *schemaGen) named(t reflect.Type) *jsonObj {
	o := &jsonObj{}
	o.set("type", "object")
	props := &jsonObj{}
	props.set("$schema", jsonObj{kv: []jsonKV{{"type", "string"}}})
	o.set("properties", props)
	pp := &jsonObj{}
	pp.set(namePattern, g.ref(t))
	o.set("patternProperties", pp)
	o.set("additionalProperties", false)
	return o
}

// ref gives a $ref to the definition of the struct type t.
func (g *schemaGen) ref(t reflect.Type) *jsonObj {
	name := strings.ToLower(t.Name()[:1]) + t.Name()[1:]
	if _, ok := g.defs[name]; !ok {
		g.defs[name] = nil
		g.defs[name] = g.object(t)
	}
	g.used[name] = true
	// A definition can refer to other definitions.
	g.markUsed(g.defs[name])
	r := &jsonObj{}
	r.set("$ref", "#/$defs/"+name)
	return r
}

// markUsed marks the definitions that the schema v refers to, also through
// other definitions.
func (g *schemaGen) markUsed(v any) {
	switch v := v.(type) {
	case *jsonObj:
		if v == nil {
			return
		}
		if r, ok := v.get("$ref").(string); ok {
			n := strings.TrimPrefix(r, "#/$defs/")
			if !g.used[n] {
				g.used[n] = true
				g.markUsed(g.defs[n])
			}
		}
		for _, kv := range v.kv {
			g.markUsed(kv.v)
		}
	case jsonObj:
		g.markUsed(&v)
	case []any:
		for _, it := range v {
			g.markUsed(it)
		}
	}
}

// object gives the schema of the struct type t.
func (g *schemaGen) object(t reflect.Type) *jsonObj {
	o := &jsonObj{}
	o.set("type", "object")
	props := &jsonObj{}
	var required []string
	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.Anonymous {
				walk(f.Type)
				continue
			}
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if name == "" || name == "-" {
				continue
			}
			tags := parseSchemaTag(f.Tag.Get("schema"))
			if _, ok := tags["required"]; ok {
				required = append(required, name)
			}
			s := g.field(f.Type, tags)
			if d := f.Tag.Get("doc"); d != "" {
				s.kv = append([]jsonKV{{"description", d}}, s.kv...)
			}
			props.set(name, s)
		}
	}
	walk(t)
	o.set("properties", props)
	if len(required) > 0 {
		o.set("required", required)
	}
	o.set("additionalProperties", false)
	return o
}

// field gives the schema of a field type with its schema tags.
func (g *schemaGen) field(t reflect.Type, tags map[string]string) *jsonObj {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	s := &jsonObj{}
	switch {
	case t == reflect.TypeOf(Cutoff{}):
		n := &jsonObj{}
		n.set("type", "integer")
		n.set("minimum", 0)
		c := &jsonObj{}
		c.set("const", "none")
		s.set("oneOf", []any{n, c})
	case t == reflect.TypeOf(Ref[Target]{}) || t == reflect.TypeOf(Ref[Action]{}):
		n := &jsonObj{}
		n.set("type", "string")
		n.set("pattern", namePattern)
		s.set("oneOf", []any{n, g.ref(t.Field(1).Type.Elem())})
	case t.Kind() == reflect.String:
		s.set("type", "string")
		if e, ok := tags["enum"]; ok {
			s.set("enum", schemaEnums(e))
		}
		if p, ok := tags["pattern"]; ok {
			s.set("pattern", schemaPatterns[p])
		}
	case t.Kind() == reflect.Int:
		s.set("type", "integer")
		if v, ok := tags["const"]; ok {
			n, _ := strconv.Atoi(v)
			s.set("const", n)
		}
		if _, ok := tags["const"]; !ok {
			min := 0
			if v, ok := tags["min"]; ok {
				min, _ = strconv.Atoi(v)
			}
			s.set("minimum", min)
		}
		if v, ok := tags["max"]; ok {
			n, _ := strconv.Atoi(v)
			s.set("maximum", n)
		}
	case t.Kind() == reflect.Bool:
		s.set("type", "boolean")
	case t.Kind() == reflect.Slice:
		s.set("type", "array")
		it := &jsonObj{}
		if t.Elem().Kind() == reflect.Struct {
			it = g.ref(t.Elem())
		} else {
			it.set("type", "string")
			if e, ok := tags["items"]; ok {
				it.set("enum", schemaEnums(e))
			} else if p, ok := tags["pattern"]; ok {
				it.set("pattern", schemaPatterns[p])
			} else {
				it.set("minLength", 1)
			}
		}
		s.set("items", it)
		s.set("minItems", 1)
	case t.Kind() == reflect.Map && t.Elem().Kind() == reflect.String:
		s.set("type", "object")
		v := &jsonObj{}
		v.set("type", "string")
		v.set("pattern", schemaPatterns[tags["values"]])
		pp := &jsonObj{}
		pp.set(schemaPatterns[tags["keys"]], v)
		s.set("patternProperties", pp)
		s.set("additionalProperties", false)
	case t.Kind() == reflect.Map:
		s.set("type", "object")
		pp := &jsonObj{}
		pp.set(namePattern, g.ref(t.Elem()))
		s.set("patternProperties", pp)
		s.set("additionalProperties", false)
	case t.Kind() == reflect.Struct:
		return g.ref(t)
	default:
		panic(fmt.Sprintf("no schema for %s", t))
	}
	return s
}

func parseSchemaTag(tag string) map[string]string {
	m := map[string]string{}
	if tag == "" {
		return m
	}
	for _, p := range strings.Split(tag, ",") {
		k, v, _ := strings.Cut(p, "=")
		m[k] = v
	}
	return m
}

// jsonObj is a JSON object that keeps the key order.
type jsonObj struct{ kv []jsonKV }

type jsonKV struct {
	k string
	v any
}

func (o *jsonObj) set(k string, v any) {
	for i := range o.kv {
		if o.kv[i].k == k {
			o.kv[i].v = v
			return
		}
	}
	o.kv = append(o.kv, jsonKV{k, v})
}

func (o *jsonObj) get(k string) any {
	for _, kv := range o.kv {
		if kv.k == k {
			return kv.v
		}
	}
	return nil
}

// MarshalJSON implements json.Marshaler.
func (o jsonObj) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, kv := range o.kv {
		if i > 0 {
			b.WriteByte(',')
		}
		k, _ := marshalJSON(kv.k)
		b.Write(k)
		b.WriteByte(':')
		v, err := marshalJSON(kv.v)
		if err != nil {
			return nil, err
		}
		b.Write(v)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// marshalJSON is json.Marshal without the HTML escapes.
func marshalJSON(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}
