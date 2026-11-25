package spec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// Version is the "spec" value of a v2 strategy file.
const Version = 2

// GameFilterPort is the port item that the Game Filter ports replace.
const GameFilterPort = "$gameFilter"

// Bundle is a named set of list refs (strategies/lists.json). A bundle in
// Target.Lists holds hostlist and ipset refs. A bundle in Target.Exclude
// holds hostlistExclude and ipsetExclude refs.
type Bundle struct {
	Hostlist        []string `json:"hostlist,omitempty" doc:"--hostlist= refs (lists:, user: or bin: path)"`
	HostlistExclude []string `json:"hostlistExclude,omitempty" doc:"--hostlist-exclude= refs"`
	Ipset           []string `json:"ipset,omitempty" doc:"--ipset= refs"`
	IpsetExclude    []string `json:"ipsetExclude,omitempty" doc:"--ipset-exclude= refs"`
}

// GameBundle is a bundle of a game module. Seed is the stock file that
// makes the user file of the bundle when it is missing.
type GameBundle struct {
	Bundle
	Seed string `json:"seed,omitempty" doc:"lists: file that makes the user: list when it is missing"`
}

// Target is a traffic selector (strategies/targets.json).
type Target struct {
	TCP            []string `json:"tcp,omitempty" doc:"TCP ports: N or N-M; $gameFilter is the Game Filter ports" schema:"pattern=port"`
	UDP            []string `json:"udp,omitempty" doc:"UDP ports: N or N-M; $gameFilter is the Game Filter ports" schema:"pattern=port"`
	L3             string   `json:"l3,omitempty" doc:"--filter-l3= (ipv4 or ipv6)"`
	L7             []string `json:"l7,omitempty" doc:"--filter-l7= names" schema:"items=l7"`
	Lists          []string `json:"lists,omitempty" doc:"bundle names with hostlist / ipset refs" schema:"pattern=name"`
	Exclude        []string `json:"exclude,omitempty" doc:"bundle names with hostlistExclude / ipsetExclude refs" schema:"pattern=name"`
	Domains        []string `json:"domains,omitempty" doc:"--hostlist-domains="`
	ExcludeDomains []string `json:"excludeDomains,omitempty" doc:"--hostlist-exclude-domains="`
	AnyProtocol    bool     `json:"anyProtocol,omitempty" doc:"winws --dpi-desync-any-protocol=1; winws2 --payload=all"`
	Payload        string   `json:"payload,omitempty" doc:"winws2 --payload= group; only when the group from l7 / anyProtocol is not correct"`
}

// Action is a desync preset (strategies/actions.json).
type Action struct {
	Desync            []string `json:"desync" doc:"attacks, in order" schema:"required,items=desync"`
	Repeats           *int     `json:"repeats,omitempty" doc:"--dpi-desync-repeats= / repeats="`
	TTL               *int     `json:"ttl,omitempty" doc:"--dpi-desync-ttl= / ip_ttl=" schema:"min=1,max=255"`
	Cutoff            *Cutoff  `json:"cutoff,omitempty" doc:"number of packets (winws nN) or \"none\""`
	SplitPos          string   `json:"splitPos,omitempty" doc:"--dpi-desync-split-pos= / pos="`
	Seqovl            *int     `json:"seqovl,omitempty" doc:"--dpi-desync-split-seqovl= / seqovl="`
	SeqovlPattern     string   `json:"seqovlPattern,omitempty" doc:"bin:, lists: or user: file"`
	Fooling           []string `json:"fooling,omitempty" doc:"fooling names" schema:"items=fooling"`
	BadseqIncrement   *int     `json:"badseqIncrement,omitempty" doc:"--dpi-desync-badseq-increment= / tcp_seq="`
	IPID              string   `json:"ipId,omitempty" doc:"--ip-id= / ip_id="`
	HostfakesplitMod  string   `json:"hostfakesplitMod,omitempty" doc:"--dpi-desync-hostfakesplit-mod="`
	FakedsplitPattern string   `json:"fakedsplitPattern,omitempty" doc:"--dpi-desync-fakedsplit-pattern= (0x<hex> or file)"`
	TLSMod            string   `json:"tlsMod,omitempty" doc:"--dpi-desync-fake-tls-mod= / tls_mod="`
	LuaPayload        string   `json:"luaPayload,omitempty" doc:"winws2 payload= on each Lua attack"`
	Fake              *Fakes   `json:"fake,omitempty" doc:"fake payloads per slot"`
}

// Fakes holds the fake refs per slot. A slot that is not in the JSON is not
// set. A set slot must have refs.
type Fakes struct {
	QUIC       []string `json:"quic,omitempty" doc:"--dpi-desync-fake-quic="`
	TLS        []string `json:"tls,omitempty" doc:"--dpi-desync-fake-tls=; ! is the stock hello"`
	HTTP       []string `json:"http,omitempty" doc:"--dpi-desync-fake-http="`
	Discord    []string `json:"discord,omitempty" doc:"--dpi-desync-fake-discord="`
	STUN       []string `json:"stun,omitempty" doc:"--dpi-desync-fake-stun="`
	Unknown    []string `json:"unknown,omitempty" doc:"--dpi-desync-fake-unknown="`
	UnknownUDP []string `json:"unknownUdp,omitempty" doc:"--dpi-desync-fake-unknown-udp="`
}

// Cutoff is the cutoff of an action: a number of packets or none. The
// number N is the nN of winws.
type Cutoff struct {
	N    int
	None bool
}

// String gives the v0.1 cutoff text: nN or none.
func (c Cutoff) String() string {
	if c.None {
		return "none"
	}
	return "n" + strconv.Itoa(c.N)
}

// MarshalJSON implements json.Marshaler.
func (c Cutoff) MarshalJSON() ([]byte, error) {
	if c.None {
		return []byte(`"none"`), nil
	}
	return []byte(strconv.Itoa(c.N)), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *Cutoff) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		if s != "none" {
			return fmt.Errorf("cutoff must be a number or \"none\": %q", s)
		}
		*c = Cutoff{None: true}
		return nil
	}
	n, err := strconv.Atoi(string(b))
	if err != nil || n < 0 {
		return fmt.Errorf("cutoff must be a number or \"none\": %s", b)
	}
	*c = Cutoff{N: n}
	return nil
}

// Ref is a reference to a named item or an inline item.
type Ref[T any] struct {
	Name   string
	Inline *T
}

// MarshalJSON implements json.Marshaler.
func (r Ref[T]) MarshalJSON() ([]byte, error) {
	if r.Inline != nil {
		return json.Marshal(r.Inline)
	}
	return json.Marshal(r.Name)
}

// UnmarshalJSON implements json.Unmarshaler. A string is a name. An object
// is an inline item. Unknown fields are an error.
func (r *Ref[T]) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	switch {
	case len(b) > 0 && b[0] == '"':
		*r = Ref[T]{}
		if err := json.Unmarshal(b, &r.Name); err != nil {
			return err
		}
		if isBlank(r.Name) {
			return errors.New("name is empty")
		}
		return nil
	case len(b) > 0 && b[0] == '{':
		var v T
		if err := decodeStrict(b, &v); err != nil {
			return err
		}
		*r = Ref[T]{Inline: &v}
		return nil
	}
	return fmt.Errorf("must be a name or an object: %s", b)
}

// Rule is one rule of a strategy or a game: a target and an action.
type Rule struct {
	Target Ref[Target] `json:"target" doc:"target name or inline target" schema:"required"`
	Action Ref[Action] `json:"action" doc:"action name or inline action" schema:"required"`
}

// CatalogInfo is the JSON of catalog.json, in the root of a catalog folder.
type CatalogInfo struct {
	Schema string `json:"$schema,omitempty"`
	ID     string `json:"id" doc:"catalog id: stock for the catalog of zapman" schema:"required,pattern=name"`
	Title  string `json:"title" doc:"name in the UI" schema:"required"`
	Spec   int    `json:"spec" doc:"format version: 2" schema:"required,const=2"`
}

// StrategyFile is the JSON of strategies/<origin>/<name>.json.
type StrategyFile struct {
	Schema   string `json:"$schema,omitempty"`
	Spec     int    `json:"spec" doc:"format version: 2" schema:"required,const=2"`
	Upstream string `json:"upstream,omitempty" doc:"flowseal/ only: the first upstream tag with the last change of the .bat file" schema:"pattern=tag"`
	Rev      int    `json:"rev,omitempty" doc:"flowseal/ only: 1 + the number of zapman changes of the argv since the upstream tag" schema:"min=1"`
	Version  int    `json:"version,omitempty" doc:"zapman/ only: the version of the strategy; add 1 when the argv changes" schema:"min=1"`
	About    string `json:"about,omitempty" doc:"a short text for people"`
	Rules    []Rule `json:"rules" doc:"rules in winws order (the first match wins)" schema:"required"`
}

// GameFile is the JSON of strategies/games/<id>.json.
type GameFile struct {
	Schema  string                `json:"$schema,omitempty"`
	ID      string                `json:"id" doc:"same as the file name" schema:"required"`
	Title   string                `json:"title" doc:"name in the UI" schema:"required"`
	Version int                   `json:"version" doc:"the version of the game module; add 1 when the argv changes" schema:"required,min=1"`
	Order   int                   `json:"order" doc:"games with a lower order go first" schema:"required"`
	Lists   map[string]GameBundle `json:"lists,omitempty" doc:"bundles of this game"`
	Updater string                `json:"updater,omitempty" doc:"list updater" schema:"enum=updater"`
	Rules   []Rule                `json:"rules" doc:"rules; they go before the strategy rules" schema:"required"`
}

// decodeStrict decodes one JSON value. Unknown fields and data after the
// value are an error.
func decodeStrict(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("data after the JSON value")
	}
	return nil
}
