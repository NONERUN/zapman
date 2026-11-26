package spec

import (
	"fmt"
	"strings"
)

// defaultTLSFake is the winws2 blob for the '!' ref. winws has a built-in
// TLS hello for '!'; winws2 has none. Each catalog must have the file
// DefaultTLSFakeFile.
const defaultTLSFake = PrefixCat + DefaultTLSFakeFile

// DefaultTLSFakeFile is the path in the catalog of the winws2 blob for the
// '!' ref.
const DefaultTLSFakeFile = FakesDir + "/tls_clienthello_www_google_com.bin"

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

// blobStem gives the blob name of a 0x<hex> ref or of a path ref: hex_<hex>
// or the file name without the extension. The name of an active:X ref is
// ACTIVE_X: it does not change when the user selects another fake.
func blobStem(ref string) (string, error) {
	if isHexRef(ref) {
		return "hex_" + ref[2:], nil
	}
	pre, name, ok := splitRef(ref)
	if !ok {
		return "", fmt.Errorf("Blob ref is not 0x<hex> or a %s path: %s", refPrefixText, ref)
	}
	if pre == PrefixActive {
		return activeStem(name), nil
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
