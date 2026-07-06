package source

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var (
	pathPattern = regexp.MustCompile(`[A-Za-z0-9_./-]+\.[A-Za-z0-9]+`)
	wordPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_/-]{2,}`)
)

type searchQuery struct {
	terms map[string]float64
	paths map[string]bool
}

type scoredTag struct {
	Tag
	score float64
}

// SearchTags reads all cached files and tags from SQLite, scoring them against the query.
func (c *Cache) SearchTags(inputQuery string, limit int) ([]Tag, error) {
	if limit <= 0 {
		limit = 10
	}

	query := buildQuery(inputQuery)
	if len(query.terms) == 0 && len(query.paths) == 0 {
		return nil, nil
	}

	rows, err := c.db.Query(`SELECT filepath, tags_json FROM file_cache`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []scoredTag

	for rows.Next() {
		var relPath string
		var tagsJSON string
		if err := rows.Scan(&relPath, &tagsJSON); err != nil {
			continue
		}

		fileScore := scoreText(relPath, query, 2.5)
		if query.hasExplicitPath(relPath) {
			fileScore += 8.0
		}

		var fileTags []Tag
		if err := json.Unmarshal([]byte(tagsJSON), &fileTags); err != nil {
			continue
		}

		for _, tag := range fileTags {
			if tag.Kind != "def" {
				continue // Only fetch snippet bodies of definitions
			}
			
			tagScore := fileScore + scoreText(tag.Name, query, 3.0)
			
			if tagScore > 0 {
				candidates = append(candidates, scoredTag{
					Tag:   tag,
					score: tagScore,
				})
			}
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	results := make([]Tag, 0, len(candidates))
	for _, c := range candidates {
		results = append(results, c.Tag)
	}

	return results, nil
}

func buildQuery(input string) searchQuery {
	query := searchQuery{
		terms: map[string]float64{},
		paths: map[string]bool{},
	}
	for _, match := range pathPattern.FindAllString(input, -1) {
		clean := filepath.ToSlash(filepath.Clean(match))
		if clean == "." || strings.HasPrefix(clean, "../") || filepath.IsAbs(clean) {
			continue
		}
		query.paths[clean] = true
		for _, term := range tokenize(clean) {
			query.terms[term] += 1.5
		}
	}
	for _, match := range wordPattern.FindAllString(input, -1) {
		for _, term := range tokenize(match) {
			if isStopWord(term) {
				continue
			}
			query.terms[term]++
		}
	}
	return query
}

func tokenize(value string) []string {
	var tokens []string
	var current strings.Builder
	flush := func() {
		if current.Len() == 0 {
			return
		}
		token := strings.ToLower(current.String())
		if len(token) >= 3 {
			tokens = append(tokens, token)
		}
		current.Reset()
	}

	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			current.WriteRune(unicode.ToLower(r))
		case r == '_' || r == '-' || r == '/' || r == '.':
			flush()
		case unicode.IsUpper(r):
			flush()
			current.WriteRune(unicode.ToLower(r))
		default:
			flush()
		}
	}
	flush()
	return tokens
}

func scoreText(value string, query searchQuery, weight float64) float64 {
	if len(query.terms) == 0 {
		return 0
	}
	text := strings.ToLower(value)
	score := 0.0
	for term, termWeight := range query.terms {
		if strings.Contains(text, term) {
			score += termWeight * weight
		}
	}
	return score
}

func (q searchQuery) hasExplicitPath(rel string) bool {
	if q.paths[rel] {
		return true
	}
	for path := range q.paths {
		if strings.HasSuffix(rel, path) || strings.HasSuffix(path, rel) {
			return true
		}
	}
	return false
}

func isStopWord(term string) bool {
	switch term {
	case "the", "and", "for", "with", "this", "that", "from", "into", "task", "title", "description", "implement", "create", "update", "fix", "need", "needs":
		return true
	default:
		return false
	}
}
