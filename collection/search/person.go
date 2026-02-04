package search

import (
	"context"
	"sort"
	"strings"

	bleve "github.com/blevesearch/bleve/v2"
)

// SearchPersons searches for person names (actors, writers, directors) and returns
// a deduplicated list of matching person names.
// - personName: the name of the person to search for
// - size: maximum number of search results to process (items, not person names)
func (b *Search) SearchPerson(ctx context.Context, personName string, size int) ([]string, error) {
	personName = strings.ToLower(strings.TrimSpace(personName))
	if personName == "" {
		return nil, nil
	}

	// Boost weights for person searches
	const (
		boostExactPhrase = 20.0 // exact phrase match (e.g., "Tom Hanks")
		boostMatchQuery  = 10.0 // match query (handles individual tokens well)
		boostPrefix      = 8.0  // prefix match (for autocomplete-style searches)
		boostFuzzy       = 3.0  // fuzzy match (for typos/partial names)
	)

	boolQuery := bleve.NewBooleanQuery()

	// 1) Exact phrase match - highest priority
	phraseQuery := bleve.NewMatchPhraseQuery(personName)
	phraseQuery.SetField(peopleField)
	phraseQuery.SetBoost(boostExactPhrase)
	boolQuery.AddShould(phraseQuery)

	// 2) Match query - good for matching full names or partial names
	matchQuery := bleve.NewMatchQuery(personName)
	matchQuery.SetField(peopleField)
	matchQuery.SetBoost(boostMatchQuery)
	boolQuery.AddShould(matchQuery)

	// 3) Prefix queries for partial name matching
	prefixQuery := bleve.NewPrefixQuery(personName)
	prefixQuery.SetField(peopleField)
	prefixQuery.SetBoost(boostPrefix)
	boolQuery.AddShould(prefixQuery)

	// 4) Token-wise fuzzy matching
	tokens := strings.FieldsSeq(personName)
	for tok := range tokens {
		if len(tok) < 2 {
			continue
		}

		// Determine fuzziness based on token length
		fuzz := 1
		if len(tok) >= 6 {
			fuzz = 2
		}

		// Fuzzy query on people field
		fuzzyQuery := bleve.NewFuzzyQuery(tok)
		fuzzyQuery.SetField(peopleField)
		fuzzyQuery.SetFuzziness(fuzz)
		fuzzyQuery.SetBoost(boostFuzzy)
		boolQuery.AddShould(fuzzyQuery)

		// Prefix query per token
		tokenPrefixQuery := bleve.NewPrefixQuery(tok)
		tokenPrefixQuery.SetField(peopleField)
		tokenPrefixQuery.SetBoost(boostPrefix)
		boolQuery.AddShould(tokenPrefixQuery)
	}

	// Require at least one condition to match
	boolQuery.SetMinShould(1)

	// Build search request - request people field
	req := bleve.NewSearchRequestOptions(boolQuery, size, 0, true)
	req.Fields = []string{peopleField}
	req.SortBy([]string{"-_score"})

	// Run query
	res, err := b.index.SearchInContext(ctx, req)
	if err != nil {
		return nil, err
	}

	// Collect all matching person names from results
	personNamesMap := make(map[string]bool) // Use map for deduplication

	for _, hit := range res.Hits {
		// Extract people field
		if peopleField, ok := hit.Fields[peopleField]; ok {
			people := extractStringArray(peopleField)

			// Filter to only include names that match the search term
			matchedNames := filterMatchingNames(people, personName)
			for _, name := range matchedNames {
				personNamesMap[name] = true
			}
		}
	}

	// Convert map to sorted slice
	personNames := make([]string, 0, len(personNamesMap))
	for name := range personNamesMap {
		personNames = append(personNames, name)
	}

	// Sort alphabetically for consistent results
	sort.Strings(personNames)

	return personNames, nil
}

// extractStringArray converts a field value to []string.
// Bleve can return arrays as []interface{} or single strings.
func extractStringArray(field any) []string {
	switch v := field.(type) {
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	case []string:
		return v
	case string:
		return []string{v}
	default:
		return []string{}
	}
}

// filterMatchingNames filters a list of names to only include those matching the search term.
// This handles partial matches, case-insensitive matching.
func filterMatchingNames(names []string, searchTerm string) []string {
	if len(names) == 0 {
		return nil
	}

	searchLower := strings.ToLower(searchTerm)
	searchTokens := strings.Fields(searchLower)

	var matched []string
	for _, name := range names {
		nameLower := strings.ToLower(name)
		nameTokens := strings.Fields(nameLower)

		// Check if the name matches:
		// 1. Contains the full search term as substring
		if strings.Contains(nameLower, searchLower) {
			matched = append(matched, name)
			continue
		}

		// 2. All search tokens appear in the name
		allTokensMatch := true
		for _, searchToken := range searchTokens {
			tokenFound := false
			for _, nameToken := range nameTokens {
				if strings.Contains(nameToken, searchToken) || strings.HasPrefix(nameToken, searchToken) {
					tokenFound = true
					break
				}
			}
			if !tokenFound {
				allTokensMatch = false
				break
			}
		}
		if allTokensMatch {
			matched = append(matched, name)
		}
	}
	return matched
}
