package search

import (
	"context"
	"strings"

	bleve "github.com/blevesearch/bleve/v2"
)

// SearchItem runs a flexible fuzzy search across several fields.
// - searchTerm is the raw user input.
// - size is maximum number of results to return.
func (b *Search) SearchItem(ctx context.Context, searchTerm string, size int) ([]string, error) {
	searchTerm = strings.ToLower(strings.TrimSpace(searchTerm))
	if searchTerm == "" {
		return nil, nil
	}

	// Weights for boosting certain query types and fields.
	const (
		boostNameExact       = 50.0 // strongest: exact match on name_exact field
		boostNamePhrase      = 12.0 // very strong: exact phrase in name
		boostNamePrefix      = 6.0  // very strong: prefix on whole query against name
		boostNameTokenPrefix = 5.0  // strong: prefix on first token against name
		boostNameField       = 3.0  // strong: fuzzy/prefix on name tokens
		boostOtherFields     = 1.0  // default for other fields
	)

	boolQuery := bleve.NewBooleanQuery()

	// 0) Exact-match: TermQuery on name_exact (keyword) with very large boost.
	// This will bubble exact title matches to the top.
	termExact := bleve.NewTermQuery(searchTerm)
	termExact.SetField(NameExactField)
	termExact.SetBoost(boostNameExact)
	boolQuery.AddShould(termExact)

	// 1) High-boost phrase match on name (exact phrase)
	matchPhrase := bleve.NewMatchPhraseQuery(searchTerm)
	matchPhrase.SetField(nameField)
	matchPhrase.SetBoost(boostNamePhrase)
	boolQuery.AddShould(matchPhrase)

	// 2) Very-high-boost prefix on the full query against name.
	// This helps when users type the beginning of a title: "star wa" -> matches "Star Wars".
	prefixFull := bleve.NewPrefixQuery(searchTerm)
	prefixFull.SetField(nameField)
	prefixFull.SetBoost(boostNamePrefix)
	boolQuery.AddShould(prefixFull)

	// 3) High-boost prefix for the first token (helps partial words)
	tokens := strings.Fields(searchTerm)
	if len(tokens) > 0 {
		first := tokens[0]
		prefixFirst := bleve.NewPrefixQuery(first)
		prefixFirst.SetField(nameField)
		prefixFirst.SetBoost(boostNameTokenPrefix)
		boolQuery.AddShould(prefixFirst)
	}

	// 4) Token-wise fuzzy + prefix queries across fields
	for _, tok := range tokens {
		// Fuzziness heuristic
		fuzz := 1
		if len(tok) >= 6 {
			fuzz = 2
		}

		// Fields to search
		fields := []string{nameField, sortNameField, overviewField}
		for _, f := range fields {
			// Fuzzy query
			fq := bleve.NewFuzzyQuery(tok)
			fq.SetField(f)
			fq.SetFuzziness(fuzz)
			if f == nameField {
				fq.SetBoost(boostNameField)
			} else {
				fq.SetBoost(boostOtherFields)
			}
			boolQuery.AddShould(fq)

			// Prefix query (helps partial typing)
			pq := bleve.NewPrefixQuery(tok)
			pq.SetField(f)
			// Apply boosts, name has higher weight
			if f == nameField {
				pq.SetBoost(boostNameField)
			} else {
				pq.SetBoost(boostOtherFields)
			}
			boolQuery.AddShould(pq)
		}
	}

	// Require at least one of conditions to match
	boolQuery.SetMinShould(1)

	// Build search request
	req := bleve.NewSearchRequestOptions(boolQuery, size, 0, true)
	// Specify fields to retrieve
	req.Fields = []string{idField, nameField}
	// Sort by score descending
	req.SortBy([]string{"-_score"})

	// run query with context-aware method if available
	res, err := b.index.SearchInContext(ctx, req)
	if err != nil {
		return nil, err
	}

	// printSearchResult(res)

	var foundIDs []string
	for _, h := range res.Hits {
		// log.Printf("search: hit: %s (%f) -> %s : %s\n", h.ID, h.Score, h.Fields[idField], h.Fields[nameField])
		foundIDs = append(foundIDs, h.ID)
	}
	return foundIDs, nil
}
