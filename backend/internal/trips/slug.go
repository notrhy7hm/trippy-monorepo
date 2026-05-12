package trips

import (
	"crypto/rand"
	"encoding/base32"
	"strings"

	"github.com/gosimple/slug"
)

// MakeSlug returns a URL-safe slug derived from title plus a short random suffix.
// Suffix prevents collisions and avoids exposing creation order / counts.
func MakeSlug(title string) string {
	base := slug.Make(title)
	if base == "" {
		base = "trip"
	}
	if len(base) > 48 {
		base = base[:48]
	}
	return base + "-" + randSuffix(5)
}

func randSuffix(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	s := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))
	if len(s) > n {
		s = s[:n]
	}
	return s
}
