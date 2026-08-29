package ranking

import (
	"sort"
	"strconv"

	"studsphere/backend/internal/search/retrieval"
)

type RRFScorer struct {
	K int
}

func NewRRFScorer(k int) *RRFScorer {
	return &RRFScorer{K: k}
}

func (s *RRFScorer) Score(ranks []int) float64 {
	score := 0.0
	for _, rank := range ranks {
		if rank > 0 {
			score += 1.0 / float64(s.K+rank)
		}
	}
	return score
}

func (s *RRFScorer) RankCandidates(candidates []retrieval.Candidate, sourceRanks map[string][]int) []retrieval.Candidate {
	for i := range candidates {
		key := candidateKey(candidates[i])
		if ranks, ok := sourceRanks[key]; ok {
			candidates[i].Score = s.Score(ranks)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	return candidates
}

func candidateKey(c retrieval.Candidate) string {
	return string(c.Type) + ":" + strconv.FormatUint(uint64(c.ID), 10)
}
