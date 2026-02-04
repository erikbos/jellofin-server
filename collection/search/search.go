package search

import (
	"context"

	bleve "github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
)

// Search is the Bleve-based search index.
type Search struct {
	// index is the underlying bleve index.
	index bleve.Index
}

// name of the search doc fields, these MUST match json annotation of Document.
const (
	idField        = "id"
	parentIDField  = "parent_id"
	nameField      = "name"
	NameExactField = "name_exact"
	sortNameField  = "sort_name"
	overviewField  = "overview"
	genresField    = "genres"
	peopleField    = "people"
)

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
	People    []string `json:"people"`
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

	// text mapping for name, overview
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

	// text mapping for people - STORED so we can retrieve them
	textFieldMappingStored := bleve.NewTextFieldMapping()
	textFieldMappingStored.Analyzer = "en"
	textFieldMappingStored.Store = true
	textFieldMappingStored.Index = true

	// Add fields to index
	doc.AddFieldMappingsAt(idField, keyword)
	doc.AddFieldMappingsAt(parentIDField, keyword)
	doc.AddFieldMappingsAt(nameField, textFieldMapping)
	doc.AddFieldMappingsAt(NameExactField, keyword)
	doc.AddFieldMappingsAt(sortNameField, textFieldMapping)
	doc.AddFieldMappingsAt(overviewField, textFieldMapping)
	doc.AddFieldMappingsAt(genresField, textFieldMapping)
	doc.AddFieldMappingsAt(peopleField, textFieldMappingStored)

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
