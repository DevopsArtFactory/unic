package app

import (
	"sort"
	"strings"
	"unicode"
)

type fuzzyMatch struct {
	score     int
	first     int
	positions []int
}

type fuzzyCandidate[T any] struct {
	item      T
	index     int
	match     fuzzyMatch
	matchText string
}

func applyFilter[T Filterable](items []T, query string) []T {
	query = strings.TrimSpace(query)
	if query == "" {
		return items
	}

	candidates := make([]fuzzyCandidate[T], 0, len(items))
	for i, item := range items {
		text := item.FilterText()
		match, ok := fuzzyMatchFilterText(text, query)
		if !ok {
			continue
		}
		candidates = append(candidates, fuzzyCandidate[T]{
			item:      item,
			index:     i,
			match:     match,
			matchText: text,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		if left.match.score != right.match.score {
			return left.match.score > right.match.score
		}
		if left.match.first != right.match.first {
			return left.match.first < right.match.first
		}
		if len(left.matchText) != len(right.matchText) {
			return len(left.matchText) < len(right.matchText)
		}
		return left.index < right.index
	})

	filtered := make([]T, 0, len(candidates))
	for _, candidate := range candidates {
		filtered = append(filtered, candidate.item)
	}
	return filtered
}

func fuzzyMatchFilterText(text, query string) (fuzzyMatch, bool) {
	queryTokens := strings.Fields(strings.TrimSpace(query))
	if len(queryTokens) == 0 {
		return fuzzyMatch{}, true
	}

	textTokens := splitFuzzyTokens(text)
	if len(textTokens) == 0 {
		return fuzzyMatch{}, false
	}

	totalScore := 0
	first := -1
	for _, queryToken := range queryTokens {
		var best fuzzyMatch
		found := false
		for _, textToken := range textTokens {
			match, ok := fuzzyMatchText(textToken.value, queryToken)
			if !ok {
				continue
			}
			match.first += textToken.offset
			if !found || match.score > best.score || (match.score == best.score && match.first < best.first) {
				best = match
				found = true
			}
		}
		if !found {
			return fuzzyMatch{}, false
		}
		totalScore += best.score
		if first == -1 || best.first < first {
			first = best.first
		}
	}

	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	if strings.Contains(lowerText, lowerQuery) {
		totalScore += 240
	}
	if strings.EqualFold(strings.TrimSpace(text), strings.TrimSpace(query)) {
		totalScore += 400
	}

	return fuzzyMatch{score: totalScore, first: first}, true
}

func fuzzyMatchText(text, query string) (fuzzyMatch, bool) {
	normalizedQuery := []rune(strings.ToLower(strings.TrimSpace(query)))
	if len(normalizedQuery) == 0 {
		return fuzzyMatch{}, true
	}

	normalizedText := []rune(strings.ToLower(text))
	positions := make([]int, 0, len(normalizedQuery))
	searchStart := 0
	score := 0
	first := -1
	lastPos := -1

	for _, want := range normalizedQuery {
		found := -1
		for idx := searchStart; idx < len(normalizedText); idx++ {
			if normalizedText[idx] == want {
				found = idx
				break
			}
		}
		if found == -1 {
			return fuzzyMatch{}, false
		}

		positions = append(positions, found)
		if first == -1 {
			first = found
			score += max(80-found*2, 0)
			if found == 0 {
				score += 80
			}
			if isFuzzyWordBoundary(normalizedText, found) {
				score += 35
			}
		} else {
			gap := found - lastPos - 1
			score += max(24-gap*3, 0)
			if gap == 0 {
				score += 45
			} else if gap == 1 {
				score += 15
			}
			if isFuzzyWordBoundary(normalizedText, found) {
				score += 15
			}
		}
		lastPos = found
		searchStart = found + 1
	}

	if len(positions) > 0 {
		span := positions[len(positions)-1] - positions[0] + 1
		if !strings.Contains(strings.ToLower(text), strings.ToLower(query)) && span > len(normalizedQuery)*2+1 {
			return fuzzyMatch{}, false
		}
		score += max(140-(span-len(normalizedQuery))*24, 0)
	}

	if strings.Contains(strings.ToLower(text), strings.ToLower(query)) {
		score += 120
	}
	if strings.EqualFold(strings.TrimSpace(text), strings.TrimSpace(query)) {
		score += 240
	}

	return fuzzyMatch{
		score:     score,
		first:     first,
		positions: positions,
	}, true
}

func isFuzzyWordBoundary(text []rune, idx int) bool {
	if idx <= 0 {
		return true
	}
	prev := text[idx-1]
	return !unicode.IsLetter(prev) && !unicode.IsDigit(prev)
}

func renderHighlightedMatch(text, query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return text
	}

	match, ok := fuzzyMatchText(text, query)
	if !ok || len(match.positions) == 0 {
		return text
	}

	runes := []rune(text)
	highlighted := make(map[int]struct{}, len(match.positions))
	for _, pos := range match.positions {
		highlighted[pos] = struct{}{}
	}

	var out strings.Builder
	for start := 0; start < len(runes); {
		_, isHighlighted := highlighted[start]
		end := start + 1
		for end < len(runes) {
			_, nextHighlighted := highlighted[end]
			if nextHighlighted != isHighlighted {
				break
			}
			end++
		}

		segment := string(runes[start:end])
		if isHighlighted {
			out.WriteString(renderHighlightSegment(segment))
		} else {
			out.WriteString(segment)
		}
		start = end
	}
	return out.String()
}

type fuzzyToken struct {
	value  string
	offset int
}

func splitFuzzyTokens(text string) []fuzzyToken {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return nil
	}

	tokens := make([]fuzzyToken, 0, len(fields))
	searchFrom := 0
	for _, field := range fields {
		offset := strings.Index(strings.ToLower(text[searchFrom:]), strings.ToLower(field))
		if offset < 0 {
			offset = 0
		}
		offset += searchFrom
		tokens = append(tokens, fuzzyToken{
			value:  field,
			offset: offset,
		})
		searchFrom = offset + len(field)
	}
	return tokens
}

func renderHighlightSegment(segment string) string {
	styled := matchStyle.Render(segment)
	if styled == segment {
		return "\x1b[1;4m" + segment + "\x1b[0m"
	}
	return styled
}
