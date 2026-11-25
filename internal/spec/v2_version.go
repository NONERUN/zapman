package spec

import (
	"fmt"
	"regexp"
	"strconv"
)

// tagPattern is the pattern of an upstream tag, for example 1.10.4.
const tagPattern = `^[0-9A-Za-z][0-9A-Za-z._-]*$`

var tagRe = regexp.MustCompile(tagPattern)

// checkVersion checks the version fields of a strategy for its origin. A
// flowseal/ strategy has upstream and rev. A zapman/ strategy has version.
func (s *Strategy) checkVersion() error {
	f := &s.StrategyFile
	switch s.origin {
	case OriginFlowseal:
		if f.Version != 0 {
			return fmt.Errorf("%s: version is for %s/ strategies. Use upstream and rev.", s.file, OriginZapman)
		}
		if !tagRe.MatchString(f.Upstream) {
			return fmt.Errorf("%s: upstream must be an upstream tag, for example 1.10.4: %q", s.file, f.Upstream)
		}
		if f.Rev < 1 {
			return fmt.Errorf("%s: rev must be 1 or more", s.file)
		}
	case OriginZapman:
		if f.Upstream != "" || f.Rev != 0 {
			return fmt.Errorf("%s: upstream and rev are for %s/ strategies. Use version.", s.file, OriginFlowseal)
		}
		if f.Version < 1 {
			return fmt.Errorf("%s: version must be 1 or more", s.file)
		}
	}
	return nil
}

// Version gives the origin and the version of the strategy for people:
// "Flowseal 1.10.4 r2" or "Zapman 3".
func (s *Strategy) Version() string {
	if s.origin == OriginFlowseal {
		return fmt.Sprintf("%s %s r%d", s.origin.Title(), s.Upstream, s.Rev)
	}
	return s.origin.Title() + " " + strconv.Itoa(s.StrategyFile.Version)
}

// Origin gives the origin of a game module. Game modules are zapman data.
func (g *Game) Origin() Origin { return OriginZapman }

// Version gives the origin and the version of the game module for people:
// "Zapman 1".
func (g *Game) Version() string {
	return g.Origin().Title() + " " + strconv.Itoa(g.GameFile.Version)
}
