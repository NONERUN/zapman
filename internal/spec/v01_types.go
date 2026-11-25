package spec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// V01Version is the specVersion of the v0.1 format.
const V01Version = "0.1.0"

// V01Spec is the root of a v0.1 strategy JSON file.
type V01Spec struct {
	SpecVersion string       `json:"specVersion"`
	WF          *V01WF       `json:"wf"`
	Profiles    []V01Profile `json:"profiles"`
}

// V01WF is the WinDivert port filter of the root.
type V01WF struct {
	TCP StringList `json:"tcp"`
	UDP StringList `json:"udp"`
}

// V01Profile is one winws profile (the part between two --new).
type V01Profile struct {
	L3  string `json:"l3"`
	L7  string `json:"l7"`
	TCP string `json:"tcp"`
	UDP string `json:"udp"`

	// Payload is the winws2 --payload= group. winws does not use it.
	Payload string `json:"payload"`
	// LuaPayload is payload= on each winws2 Lua attack.
	LuaPayload string `json:"luaPayload"`

	Hostlist               StringList `json:"hostlist"`
	HostlistExclude        StringList `json:"hostlistExclude"`
	HostlistDomains        StringList `json:"hostlistDomains"`
	HostlistExcludeDomains StringList `json:"hostlistExcludeDomains"`
	Ipset                  StringList `json:"ipset"`
	IpsetExclude           StringList `json:"ipsetExclude"`

	Desync          StringList `json:"desync"`
	Repeats         *int       `json:"repeats"`
	AnyProtocol     bool       `json:"anyProtocol"`
	Cutoff          string     `json:"cutoff"`
	SplitPos        string     `json:"splitPos"`
	Seqovl          *int       `json:"seqovl"`
	SeqovlPattern   string     `json:"seqovlPattern"`
	Fooling         StringList `json:"fooling"`
	BadseqIncrement *int       `json:"badseqIncrement"`
	TTL             *int       `json:"ttl"`
	IPID            string     `json:"ipId"`

	TLSMod            string `json:"tlsMod"`
	HostfakesplitMod  string `json:"hostfakesplitMod"`
	FakedsplitPattern string `json:"fakedsplitPattern"`

	// DefaultTLSFake false stops the implicit TLS hello on winws2.
	DefaultTLSFake *bool    `json:"defaultTlsFake"`
	Fake           V01Fakes `json:"fake"`
}

// V01Fakes holds the fake payload refs per slot.
type V01Fakes struct {
	QUIC       FakeSlot `json:"quic"`
	TLS        FakeSlot `json:"tls"`
	HTTP       FakeSlot `json:"http"`
	Discord    FakeSlot `json:"discord"`
	STUN       FakeSlot `json:"stun"`
	Unknown    FakeSlot `json:"unknown"`
	UnknownUDP FakeSlot `json:"unknownUdp"`
}

// StringList is a JSON array of strings. A single string is a list of one
// item. Null items are skipped.
type StringList []string

// UnmarshalJSON implements json.Unmarshaler.
func (l *StringList) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if bytes.Equal(b, []byte("null")) {
		*l = nil
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*l = StringList{s}
		return nil
	}
	var items []*string
	if err := json.Unmarshal(b, &items); err != nil {
		return err
	}
	out := make(StringList, 0, len(items))
	for _, it := range items {
		if it != nil {
			out = append(out, *it)
		}
	}
	*l = out
	return nil
}

// FakeSlot is one fake slot. Set is true when the key is in the JSON, also
// with a null or empty value.
type FakeSlot struct {
	Set  bool
	Refs StringList
}

// UnmarshalJSON implements json.Unmarshaler. It is also called for null.
func (s *FakeSlot) UnmarshalJSON(b []byte) error {
	s.Set = true
	return s.Refs.UnmarshalJSON(b)
}

type fakeSlotDef struct {
	name string
	flag string
	get  func(*V01Fakes) *FakeSlot
}

// v01FakeSlots is the slot order of the module (ZapretSpecFakeSlotOrder).
var v01FakeSlots = []fakeSlotDef{
	{"quic", "--dpi-desync-fake-quic", func(f *V01Fakes) *FakeSlot { return &f.QUIC }},
	{"tls", "--dpi-desync-fake-tls", func(f *V01Fakes) *FakeSlot { return &f.TLS }},
	{"http", "--dpi-desync-fake-http", func(f *V01Fakes) *FakeSlot { return &f.HTTP }},
	{"discord", "--dpi-desync-fake-discord", func(f *V01Fakes) *FakeSlot { return &f.Discord }},
	{"stun", "--dpi-desync-fake-stun", func(f *V01Fakes) *FakeSlot { return &f.STUN }},
	{"unknown", "--dpi-desync-fake-unknown", func(f *V01Fakes) *FakeSlot { return &f.Unknown }},
	{"unknownUdp", "--dpi-desync-fake-unknown-udp", func(f *V01Fakes) *FakeSlot { return &f.UnknownUDP }},
}

// Parse reads a strategy JSON text. The specVersion must be V01Version.
// Unknown fields are an error.
func Parse(data []byte) (*V01Spec, error) {
	var head struct {
		SpecVersion json.RawMessage `json:"specVersion"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, fmt.Errorf("Strategy JSON is not valid: %w", err)
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil, errors.New("Strategy JSON is empty")
	}
	got := ""
	if len(head.SpecVersion) > 0 && !bytes.Equal(head.SpecVersion, []byte("null")) {
		if err := json.Unmarshal(head.SpecVersion, &got); err != nil {
			// The module converts a number to text, so it is only a mismatch.
			got = string(head.SpecVersion)
		}
	}
	if isBlank(got) {
		return nil, errors.New("Strategy JSON has no specVersion")
	}
	if got != V01Version {
		return nil, fmt.Errorf("Strategy JSON specVersion '%s' does not match ZapretSpec %s", got, V01Version)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var s V01Spec
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("Strategy JSON is not valid: %w", err)
	}
	return &s, nil
}

// LoadFile reads and parses a strategy JSON file.
func LoadFile(path string) (*V01Spec, error) {
	if isBlank(path) {
		return nil, errors.New("Strategy JSON path is empty.")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("Strategy JSON not found: %s", path)
		}
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	s, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, path)
	}
	return s, nil
}

// isBlank is String.IsNullOrWhiteSpace.
func isBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}
