package search

import (
	"context"
	"errors"
	"strings"

	bleve "github.com/blevesearch/bleve/v2"
)

// Similar runs a similarity query for the given item.
// - doc: metadata of the item to base similarity on. (at least ID and some fields populated)
// - size: number of similar items to return.
func (b *Search) Similar(ctx context.Context, doc Document, size int) ([]string, error) {
	if b == nil || b.index == nil || doc.ID == "" {
		return nil, errors.New("search index not initialized or invalid document")
	}

	// Weights for boosting certain query types and fields
	const (
		boostNameTokenPref = 5.0 // strongest: prefix on first token against name
		boostNameToken     = 3.0 // strong: fuzzy/prefix on name tokens
		boostGenre         = 2.0 // moderate: term query on genres
		boostOtherFields   = 1.0 // default for other fields
		boostPeople        = 2.0 // moderate: match query on people
		boostOverview      = 0.5 // lower: match query on overview
	)

	boolQuery := bleve.NewBooleanQuery()

	// 1) Must exclude similar reference item from result set.
	termSelf := bleve.NewTermQuery(doc.ID)
	termSelf.SetField(idField)
	boolQuery.AddMustNot(termSelf)

	// 2) Must restrict results to same collection if ParentID is set.
	if doc.ParentID != "" {
		cq := bleve.NewTermQuery(doc.ParentID)
		cq.SetField(parentIDField)
		boolQuery.AddMust(cq)
	}

	// 3) High-boost prefix for the first token (helps partial words)
	tokens := strings.Fields(doc.Name)
	if len(tokens) > 0 {
		first := tokens[0]
		prefixFirst := bleve.NewPrefixQuery(first)
		prefixFirst.SetField(nameField)
		prefixFirst.SetBoost(boostNameTokenPref)
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
				fq.SetBoost(boostNameToken)
			} else {
				fq.SetBoost(boostOtherFields)
			}
			boolQuery.AddShould(fq)

			// Prefix query (helps partial typing)
			pq := bleve.NewPrefixQuery(tok)
			pq.SetField(f)
			// Apply boosts, name has higher weight
			if f == nameField {
				pq.SetBoost(boostNameToken)
			} else {
				pq.SetBoost(boostOtherFields)
			}
			boolQuery.AddShould(pq)
		}
	}

	// Genres: term queries with boost
	for _, g := range doc.Genres {
		if g == "" {
			continue
		}
		tq := bleve.NewTermQuery(strings.ToLower(g))
		tq.SetField(genresField)
		tq.SetBoost(boostGenre)
		boolQuery.AddShould(tq)
	}

	// People: match queries (or term) with moderate boost
	for _, a := range doc.People {
		if a == "" {
			continue
		}
		aq := bleve.NewMatchQuery(a)
		aq.SetField(peopleField)
		aq.SetBoost(boostPeople)
		boolQuery.AddShould(aq)
	}

	// Overview/text: match query with lower boost
	if doc.Overview != "" {
		ov := bleve.NewMatchQuery(doc.Overview)
		ov.SetField(overviewField)
		ov.SetBoost(boostOverview)
		boolQuery.AddShould(ov)
	}

	req := bleve.NewSearchRequestOptions(boolQuery, size, 0, false)
	// Specify fields to retrieve, we only need IDs for now
	// req.Fields = []string{"id"}
	req.Fields = []string{idField, nameField}
	// Sort by score descending
	req.SortBy([]string{"-_score"})

	res, err := b.index.SearchInContext(ctx, req)
	if err != nil {
		return nil, err
	}

	var foundIDs []string
	for _, h := range res.Hits {
		// log.Printf("similar hit: %s (%f) -> %s : %s\n", h.ID, h.Score, h.Fields[idField], h.Fields[nameField])
		foundIDs = append(foundIDs, h.ID)
	}
	return foundIDs, nil
}
