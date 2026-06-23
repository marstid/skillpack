package skill

import (
	"sort"
	"strings"
)

// search implements the Store.Search method: case-insensitive subsequence
// matching over "name description" with a position-based score. Lower scores
// rank earlier; insertion order breaks ties. An empty query returns the full
// catalog unfiltered and unsorted (insertion order).
func search(s *Store, query string) []CatalogEntry {
	entries := s.List()
	if query == "" {
		return entries
	}
	q := strings.ToLower(query)
	type scored struct {
		entry CatalogEntry
		score int
		idx   int
	}
	var matches []scored
	for i, e := range entries {
		hay := strings.ToLower(e.Name + " " + e.Description)
		score, ok := subsequenceScore(hay, q)
		if !ok {
			continue
		}
		matches = append(matches, scored{e, score, i})
	}
	// Stable sort preserves insertion order for equal scores.
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score < matches[j].score
		}
		return matches[i].idx < matches[j].idx
	})
	out := make([]CatalogEntry, len(matches))
	for i, m := range matches {
		out[i] = m.entry
	}
	return out
}

// subsequenceScore reports whether q is a case-insensitive subsequence of hay
// and returns a score where lower is a better (tighter) match. The score
// combines:
//   - prefix penalty: the position of the first matched character (earlier is
//     better); a first-position match additionally earns prefixHitBonus.
//   - gap penalty: the number of intervening characters between consecutive
//     matched characters (tighter is better).
//   - word-boundary bonus: matches at the start of the hay or following a
//     space or hyphen earn wordBoundaryBonus (reduces the score).
//
// Returns ok=false when q is not a subsequence of hay.
func subsequenceScore(hay, q string) (int, bool) {
	if q == "" {
		return 0, true
	}
	const (
		wordBoundaryBonus = -2
		prefixHitBonus    = -3
	)
	score := 0
	prevMatch := -1
	qi := 0
	for i := 0; i < len(hay) && qi < len(q); i++ {
		if hay[i] != q[qi] {
			continue
		}
		if qi == 0 {
			score += i // prefix penalty: earlier first match is better
			if i == 0 {
				score += prefixHitBonus
			}
		} else {
			score += i - prevMatch - 1 // gap penalty
		}
		if i == 0 || hay[i-1] == ' ' || hay[i-1] == '-' {
			score += wordBoundaryBonus
		}
		prevMatch = i
		qi++
	}
	if qi != len(q) {
		return 0, false
	}
	return score, true
}
