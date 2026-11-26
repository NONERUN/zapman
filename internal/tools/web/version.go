package web

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// ErrNoVersion is the error of LatestVersion when the answer has no
// version tag.
var ErrNoVersion = errors.New("the answer of GitHub has no version tag")

// LatestVersion gives the highest tag vMAJOR.MINOR.PATCH of the repository,
// for example "v0.1.1". It waits VersionTimeout at most.
func (c *Client) LatestVersion(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, VersionTimeout)
	defer cancel()
	body, err := c.get(ctx, or(c.TagsURL, TagsURL), "application/vnd.github+json", maxVersionBody, nil)
	if err != nil {
		return "", err
	}
	v := VersionFromBody(string(body))
	if v == "" {
		return "", ErrNoVersion
	}
	return v, nil
}

// NormalizeVersion is ConvertTo-ZapmanVersion: the first line of text
// without spaces, with a "v" in front (a "V" becomes "v"). Empty text
// gives "".
func NormalizeVersion(text string) string {
	t := strings.TrimSpace(text)
	if t == "" {
		return ""
	}
	t, _, _ = strings.Cut(t, "\n")
	t = strings.TrimSpace(t)
	if t[0] == 'v' || t[0] == 'V' {
		return "v" + t[1:]
	}
	return "v" + t
}

var versionRE = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)

// ParseVersion gives the three numbers of the tag (after NormalizeVersion),
// or false when the tag is not vMAJOR.MINOR.PATCH.
func ParseVersion(tag string) ([3]int, bool) {
	var n [3]int
	m := versionRE.FindStringSubmatch(NormalizeVersion(tag))
	if m == nil {
		return n, false
	}
	for i := range n {
		v, err := strconv.Atoi(m[i+1])
		if err != nil {
			return n, false
		}
		n[i] = v
	}
	return n, true
}

// CompareVersions compares two tags as numbers: -1 when a < b, 0 when they
// are equal, +1 when a > b. A tag that is not vMAJOR.MINOR.PATCH is lower
// than all correct tags.
func CompareVersions(a, b string) int {
	na, oka := ParseVersion(a)
	nb, okb := ParseVersion(b)
	switch {
	case !oka && !okb:
		return 0
	case !oka:
		return -1
	case !okb:
		return 1
	}
	for i := range na {
		if c := cmp.Compare(na[i], nb[i]); c != 0 {
			return c
		}
	}
	return 0
}

// VersionFromBody is Get-ZapmanVersionFromRemoteBody. A JSON object gives
// its tag_name (a release). A JSON array gives the highest "name" that is
// vMAJOR.MINOR.PATCH (the tag list). Other text is a version as it is.
func VersionFromBody(body string) string {
	t := strings.TrimSpace(body)
	switch {
	case t == "":
		return ""
	case strings.HasPrefix(t, "{"):
		var rel struct {
			TagName *string `json:"tag_name"`
		}
		if err := json.Unmarshal([]byte(t), &rel); err != nil || rel.TagName == nil {
			return ""
		}
		return NormalizeVersion(*rel.TagName)
	case strings.HasPrefix(t, "["):
		var tags []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(t), &tags); err != nil {
			return ""
		}
		best := ""
		for _, tag := range tags {
			v := NormalizeVersion(tag.Name)
			if _, ok := ParseVersion(v); !ok {
				continue
			}
			if best == "" || CompareVersions(v, best) > 0 {
				best = v
			}
		}
		return best
	}
	return NormalizeVersion(t)
}
