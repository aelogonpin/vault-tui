package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
)

// substringFilter replaces bubbles/list's default fuzzy filter (which
// matches almost anything against a one- or two-letter term) with plain
// case-insensitive substring matching — what you actually want when
// narrowing down a folder/secret/version list by typing.
func substringFilter(term string, targets []string) []list.Rank {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		ranks := make([]list.Rank, len(targets))
		for i := range targets {
			ranks[i] = list.Rank{Index: i}
		}
		return ranks
	}

	var ranks []list.Rank
	for i, target := range targets {
		idx := strings.Index(strings.ToLower(target), term)
		if idx < 0 {
			continue
		}
		matched := make([]int, len(term))
		for j := range matched {
			matched[j] = idx + j
		}
		ranks = append(ranks, list.Rank{Index: i, MatchedIndexes: matched})
	}
	return ranks
}
