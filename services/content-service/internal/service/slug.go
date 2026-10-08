package service

import (
	"regexp"
	"strconv"
	"strings"
)

var slugSeparator = regexp.MustCompile(`[^a-z0-9]+`)

// slugMaxBase keeps room for "-<n>" suffixes inside the VARCHAR(250)
// slug column.
const slugMaxBase = 200

// slugify lowercases a title into a URL slug. Slugs are server-generated
// only; client input is never trusted. Un-slugifiable titles fall back
// to the provided fallback and gain uniqueness from the suffix loop.
func slugify(title string, fallback string) string {
	slug := slugSeparator.ReplaceAllString(strings.ToLower(title), "-")
	slug = strings.Trim(slug, "-")

	if len(slug) > slugMaxBase {
		slug = strings.Trim(slug[:slugMaxBase], "-")
	}

	if slug == "" {
		slug = fallback
	}

	return slug
}

// slugCandidate returns base for the first attempt and base-2, base-3,
// ... afterwards. Uniqueness is enforced by the database UNIQUE
// constraint; this loop only reduces collisions.
func slugCandidate(base string, attempt int) string {
	if attempt == 0 {
		return base
	}

	return base + "-" + strconv.Itoa(attempt+1)
}
