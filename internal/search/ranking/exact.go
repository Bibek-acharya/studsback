package ranking

import (
	"sort"
	"strings"

	"studsphere/backend/internal/search/retrieval"
)

type ExactMatchConfig struct {
	TitleExact  float64
	TitlePhrase float64
	TitlePrefix float64
	TokenMatch  float64
	DescMatch   float64
}

type ExactMatcher struct {
	config ExactMatchConfig
}

func NewExactMatcher(cfg ExactMatchConfig) *ExactMatcher {
	return &ExactMatcher{config: cfg}
}

func (m *ExactMatcher) Boost(candidates []retrieval.Candidate, query string) []retrieval.Candidate {
	queryLower := strings.ToLower(query)
	queryTokens := strings.Fields(queryLower)

	for i := range candidates {
		titleLower := strings.ToLower(candidates[i].Title)
		descLower := strings.ToLower(candidates[i].Description)

		switch {
		case titleLower == queryLower:
			candidates[i].Score += m.config.TitleExact
		case strings.HasPrefix(titleLower, queryLower):
			candidates[i].Score += m.config.TitlePrefix
		case strings.Contains(titleLower, queryLower):
			candidates[i].Score += m.config.TitlePhrase
		case allTokensIn(queryTokens, titleLower):
			candidates[i].Score += m.config.TokenMatch
		}

		if m.config.DescMatch > 0 && strings.Contains(descLower, queryLower) {
			candidates[i].Score += m.config.DescMatch
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	return candidates
}

func allTokensIn(tokens []string, text string) bool {
	for _, t := range tokens {
		if !strings.Contains(text, t) {
			return false
		}
	}
	return true
}
