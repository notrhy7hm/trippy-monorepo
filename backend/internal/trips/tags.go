package trips

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// maxTagsPerMember caps the count of distinct tags after normalization.
	maxTagsPerMember = 8
	// maxTagLength caps the rune length of a single normalized tag.
	maxTagLength = 24
)

// tagPattern matches a fully-normalized tag: lowercase ASCII letters,
// digits, dashes, and underscores. Length is checked separately so a
// too-long tag surfaces as ErrTagTooLong rather than ErrInvalidTag.
var tagPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// normalizeTags trims, lowercases, collapses internal whitespace to '-',
// validates each tag against tagPattern, and dedupes while preserving
// input order (first occurrence wins). A nil or empty input is valid and
// returns an empty slice — meaning "clear all tags".
//
// Returns ErrTagTooLong / ErrInvalidTag at the first offending tag and
// ErrTooManyTags if more than maxTagsPerMember distinct tags survive
// normalization.
func normalizeTags(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, raw := range in {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}
		t = strings.ToLower(t)
		// Collapse any internal whitespace run into a single dash.
		t = strings.Join(strings.Fields(t), "-")
		if t == "" {
			continue
		}
		if utf8.RuneCountInString(t) > maxTagLength {
			return nil, ErrTagTooLong
		}
		if !tagPattern.MatchString(t) {
			return nil, ErrInvalidTag
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	if len(out) > maxTagsPerMember {
		return nil, ErrTooManyTags
	}
	return out, nil
}
