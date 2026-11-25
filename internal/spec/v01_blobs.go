package spec

import (
	"fmt"
	"strings"
)

// defaultTLSFake is the winws2 blob for the '!' ref. winws has a built-in
// TLS hello for '!'; winws2 has none.
const defaultTLSFake = "bin:tls_clienthello_www_google_com.bin"

type blob struct {
	name  string
	value string
}

// blobRegistry is the winws2 --blob= table (Register-ZapretSpecBlobs).
// Keys ignore case, as in a PowerShell hashtable.
type blobRegistry struct {
	order []blob
	names map[string]string // lower(ref) -> blob name
	used  foldSet           // blob names
	paths Paths
}

func newBlobRegistry(p Paths) *blobRegistry {
	return &blobRegistry{names: map[string]string{}, used: foldSet{}, paths: p}
}

func (r *blobRegistry) name(ref string) (string, bool) {
	n, ok := r.names[strings.ToLower(ref)]
	return n, ok
}

func (r *blobRegistry) add(ref string) error {
	if isBlank(ref) {
		return nil
	}
	if ref == "!" {
		if err := r.add(defaultTLSFake); err != nil {
			return err
		}
		if n, ok := r.name(defaultTLSFake); ok {
			r.names["!"] = n
		}
		return nil
	}
	if _, ok := r.name(ref); ok {
		return nil
	}
	base, err := blobStem(ref)
	if err != nil {
		return err
	}
	stem := base
	for n := 2; r.used.has(stem); n++ {
		stem = fmt.Sprintf("%s_%d", base, n)
	}
	value := ref
	if p, ok := resolveRef(ref, r.paths); ok {
		value = "@" + quote(p)
	}
	r.names[strings.ToLower(ref)] = stem
	r.used.add(stem)
	r.order = append(r.order, blob{stem, value})
	return nil
}

// blobStem is Get-ZapretSpecBlobStem.
func blobStem(ref string) (string, error) {
	if s := psEnd(ref); len(s) > 2 && (s[:2] == "0x" || s[:2] == "0X") && isHex(s[2:]) {
		return "hex_" + s[2:], nil
	}
	if !isFileRef(ref) {
		var b strings.Builder
		run := false
		for _, r := range ref {
			if isASCIIAlnum(r) {
				b.WriteRune(r)
				run = false
			} else if !run {
				b.WriteByte('_')
				run = true
			}
		}
		safe := strings.Trim(b.String(), "_")
		if isBlank(safe) {
			return "blob", nil
		}
		if safe[0] >= '0' && safe[0] <= '9' {
			return "b_" + safe, nil
		}
		return safe, nil
	}
	name := ref[strings.Index(ref, ":")+1:]
	if name == "" {
		return "", fmt.Errorf("Blob ref has no file name: %s", ref)
	}
	file := leafName(name)
	if dot := strings.LastIndex(file, "."); dot > 0 {
		return file[:dot], nil
	}
	return file, nil
}

func isHex(s string) bool {
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return false
		}
	}
	return s != ""
}
