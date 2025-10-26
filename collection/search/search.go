package search

import (
	"context"
	"errors"
	"strings"

	bleve "github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
)

// Search is the Bleve-based search index.
type Search struct {
	// index is the underlying bleve index.
	index bleve.Index
}

// Document is the document we store in Bleve per item.
type Document struct {
	// Item ID
	ID string `json:"id"`
	// Parent collection ID
	ParentID string `json:"parent_id"`
	Name     string `json:"name"`
	// NameExact is helper field to make exact name match more accurate
	NameExact string   `json:"name_exact"`
	SortName  string   `json:"sort_name"`
	Overview  string   `json:"overview"`
	Genres    []string `json:"genres"`
	Actors    []string `json:"actors"`
	Year      int      `json:"year"`
}

// New creates a new in-memory index.
func New() (*Search, error) {
	var idx bleve.Index
	var err error

	mapping := buildIndexMapping()
	idx, err = bleve.NewMemOnly(mapping)
	if err != nil {
		return nil, err
	}

	return &Search{
		index: idx,
	}, nil
}

// buildIndexMapping builds the Bleve index field mapping configuration.
func buildIndexMapping() mapping.IndexMapping {
	// default mapping (standard analyzer). You can create custom analyzers if needed.
	m := bleve.NewIndexMapping()

	// Document mapping (type "doc")
	doc := bleve.NewDocumentMapping()

	// text mapping for name, overview, actors
	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.Analyzer = "en" // use english analyzer (tokenization, lowercasing)
	// Not storing the full text, only indexing. We only care about getting match and then retrieving IDs.
	textFieldMapping.Store = false
	textFieldMapping.Index = true

	// keyword mapping for exact matches like IDs
	keyword := bleve.NewTextFieldMapping()
	keyword.Analyzer = "keyword"
	keyword.Store = true
	keyword.Index = true

	// Add fields
	doc.AddFieldMappingsAt("id", keyword)
	doc.AddFieldMappingsAt("parent_id", keyword)
	doc.AddFieldMappingsAt("name", textFieldMapping)
	doc.AddFieldMappingsAt("name_exact", keyword)
	doc.AddFieldMappingsAt("sort_name", textFieldMapping)
	doc.AddFieldMappingsAt("overview", textFieldMapping)
	doc.AddFieldMappingsAt("actors", textFieldMapping)
	doc.AddFieldMappingsAt("genres", textFieldMapping)

	m.DefaultMapping = doc

	return m
}

// Index indexes or updates a document.
func (b *Search) Index(ctx context.Context, doc Document) error {
	// You can use an async batch for speed; here we index directly.
	return b.index.Index(doc.ID, doc)
}

// IndexBatch indexes a slice of documents in a single batch (faster).
func (b *Search) IndexBatch(ctx context.Context, docs []Document) error {
	batch := b.index.NewBatch()
	for _, d := range docs {
		if err := batch.Index(d.ID, d); err != nil {
			return err
		}
		// commit in big batches to avoid huge memory usage
		if batch.Size() > 1000 {
			if err := b.index.Batch(batch); err != nil {
				return err
			}
			batch = b.index.NewBatch()
		}
	}
	if batch.Size() > 0 {
		if err := b.index.Batch(batch); err != nil {
			return err
		}
	}
	return nil
}

// Search runs a flexible fuzzy search across several fields.
//
// - searchTerm is the raw user input.
// - size is maximum number of results to return.
func (b *Search) Search(ctx context.Context, searchTerm string, size int) ([]string, error) {
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
	termExact.SetField("name_exact")
	termExact.SetBoost(boostNameExact)
	boolQuery.AddShould(termExact)

	// 1) High-boost phrase match on name (exact phrase)
	matchPhrase := bleve.NewMatchPhraseQuery(searchTerm)
	matchPhrase.SetField("name")
	matchPhrase.SetBoost(boostNamePhrase)
	boolQuery.AddShould(matchPhrase)

	// 2) Very-high-boost prefix on the full query against name.
	// This helps when users type the beginning of a title: "star wa" -> matches "Star Wars".
	prefixFull := bleve.NewPrefixQuery(searchTerm)
	prefixFull.SetField("name")
	prefixFull.SetBoost(boostNamePrefix)
	boolQuery.AddShould(prefixFull)

	// 3) High-boost prefix for the first token (helps partial words)
	tokens := strings.Fields(searchTerm)
	if len(tokens) > 0 {
		first := tokens[0]
		prefixFirst := bleve.NewPrefixQuery(first)
		prefixFirst.SetField("name")
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
		fields := []string{"name", "sort_name", "overview"}
		for _, f := range fields {
			// Fuzzy query
			fq := bleve.NewFuzzyQuery(tok)
			fq.SetField(f)
			fq.SetFuzziness(fuzz)
			if f == "name" {
				fq.SetBoost(boostNameField)
			} else {
				fq.SetBoost(boostOtherFields)
			}
			boolQuery.AddShould(fq)

			// Prefix query (helps partial typing)
			pq := bleve.NewPrefixQuery(tok)
			pq.SetField(f)
			// Apply boosts, name has higher weight
			if f == "name" {
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
	req.Fields = []string{"id", "name"}
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
		// log.Printf("search: hit: %s (%f) -> %s : %s\n", h.ID, h.Score, h.Fields["id"], h.Fields["name"])
		foundIDs = append(foundIDs, h.ID)
	}
	return foundIDs, nil
}

// Similar runs a similarity query for the given item.
// - ctx: request context
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
		boostActor         = 2.0 // moderate: match query on actors
		boostOverview      = 0.5 // lower: match query on overview
	)

	boolQuery := bleve.NewBooleanQuery()

	// 1) Restrict results to same collection.
	if doc.ParentID != "" {
		cq := bleve.NewTermQuery(doc.ParentID)
		cq.SetField("parent_id")
		boolQuery.AddMust(cq)
	}

	// 2) Exclude reference item from result set.
	termSelf := bleve.NewTermQuery(doc.ID)
	termSelf.SetField("id")
	boolQuery.AddMustNot(termSelf)

	// 3) High-boost prefix for the first token (helps partial words)
	tokens := strings.Fields(doc.Name)
	if len(tokens) > 0 {
		first := tokens[0]
		prefixFirst := bleve.NewPrefixQuery(first)
		prefixFirst.SetField("name")
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
		fields := []string{"name", "sort_name", "overview"}
		for _, f := range fields {
			// Fuzzy query
			fq := bleve.NewFuzzyQuery(tok)
			fq.SetField(f)
			fq.SetFuzziness(fuzz)
			if f == "name" {
				fq.SetBoost(boostNameToken)
			} else {
				fq.SetBoost(boostOtherFields)
			}
			boolQuery.AddShould(fq)

			// Prefix query (helps partial typing)
			pq := bleve.NewPrefixQuery(tok)
			pq.SetField(f)
			// Apply boosts, name has higher weight
			if f == "name" {
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
		tq.SetField("genres")
		tq.SetBoost(boostGenre)
		boolQuery.AddShould(tq)
	}

	// Actors: match queries (or term) with moderate boost
	for _, a := range doc.Actors {
		if a == "" {
			continue
		}
		aq := bleve.NewMatchQuery(a)
		aq.SetField("actors")
		aq.SetBoost(boostActor)
		boolQuery.AddShould(aq)
	}

	// Overview/text: match query with lower boost
	if doc.Overview != "" {
		ov := bleve.NewMatchQuery(doc.Overview)
		ov.SetField("overview")
		ov.SetBoost(boostOverview)
		boolQuery.AddShould(ov)
	}

	// Require at least two query conditions to match (see first match conditions):
	// - 1. Restrict results to same collection.
	// - 2. Exclude reference item from result set.
	boolQuery.SetMinShould(2)

	req := bleve.NewSearchRequestOptions(boolQuery, size, 0, false)
	// Specify fields to retrieve, we only need IDs for now
	// req.Fields = []string{"id"}
	req.Fields = []string{"id", "name"}
	// Sort by score descending
	req.SortBy([]string{"-_score"})

	res, err := b.index.SearchInContext(ctx, req)
	if err != nil {
		return nil, err
	}

	var foundIDs []string
	for _, h := range res.Hits {
		// log.Printf("similar hit: %s (%f) -> %s : %s\n", h.ID, h.Score, h.Fields["id"], h.Fields["name"])
		foundIDs = append(foundIDs, h.ID)
	}
	return foundIDs, nil
}

// Delete removes a document from the index.
func (b *Search) Delete(ctx context.Context, id string) error {
	return b.index.Delete(id)
}

// Close closes the underlying index.
func (b *Search) Close() error {
	return b.index.Close()
}

// printSearchResult is helper to print bleve search results with explanations.
// func printSearchResult(res *bleve.SearchResult) {
// 	fmt.Printf("Total: %d  Took: %v\n", res.Total, res.Took)
// 	for i, hit := range res.Hits {
// 		fmt.Printf("\nHit %d: id=%s score=%.6f\n", i+1, hit.ID, hit.Score)
// 		// show returned fields if any
// 		if len(hit.Fields) > 0 {
// 			fmt.Printf(" Fields: ")
// 			for k, v := range hit.Fields {
// 				fmt.Printf("%s=%v ", k, v)
// 			}
// 			fmt.Println()
// 		}
// 		// print highlight fragments (optional)
// 		if len(hit.Fragments) > 0 {
// 			fmt.Println(" Fragments:")
// 			for f, frags := range hit.Fragments {
// 				fmt.Printf("  %s:\n", f)
// 				for _, frag := range frags {
// 					fmt.Printf("   %s\n", frag)
// 				}
// 			}
// 		}
// 		// print Explanation tree
// 		if hit.Expl != nil {
// 			fmt.Println(" Explanation:")
// 			printExplain(hit.Expl, "  ")
// 		} else {
// 			fmt.Println(" No explanation available")
// 		}
// 	}
// }

// func printExplain(e *search.Explanation, indent string) {
// 	if e == nil {
// 		return
// 	}
// 	// Message contains the clause description, Value is contribution (float64)
// 	fmt.Printf("%s- value=%.6f  desc=%s\n", indent, e.Value, strings.TrimSpace(e.Message))
// 	for _, c := range e.Children {
// 		printExplain(c, indent+"  ")
// 	}
// }
