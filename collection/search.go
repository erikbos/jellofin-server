package collection

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/erikbos/jellofin-server/collection/search"
)

// BuildSearchIndex builds the search index for the collection repository.
func (j *CollectionRepo) BuildSearchIndex(ctx context.Context) error {
	log.Printf("Search compiling dataset..")

	index, err := search.New()
	if err != nil {
		return err
	}

	var docs []search.Document
	for _, c := range j.collections {
		for _, i := range c.Items {
			docs = append(docs, makeSearchDocument(&c, i))
		}
	}

	log.Printf("Search initializing index..")
	err = index.IndexBatch(ctx, docs)
	if err != nil {
		return err
	}

	log.Printf("Search added %d items.", len(docs))
	j.bleveIndex = index

	return nil
}

var (
	SearchIndexNotInitializedError = errors.New("search index not initialized")
	// default number of search results to return.
	searchResultCount = 15
)

// SearchItem performs an item search in collection repository and returns matching items.
func (j *CollectionRepo) SearchItem(ctx context.Context, term string) ([]string, error) {
	if j.bleveIndex == nil {
		return nil, SearchIndexNotInitializedError
	}
	return j.bleveIndex.SearchItem(ctx, term, searchResultCount)
}

// SearchPerson performs a person search in collection repository and returns matching person names.
func (j *CollectionRepo) SearchPerson(ctx context.Context, term string) ([]string, error) {
	if j.bleveIndex == nil {
		return nil, SearchIndexNotInitializedError
	}
	return j.bleveIndex.SearchPerson(ctx, term, searchResultCount)
}

// Similar performs a item search in collection repository and returns matching items.
func (j *CollectionRepo) Similar(ctx context.Context, c *Collection, i Item) ([]string, error) {
	if j.bleveIndex == nil {
		return nil, SearchIndexNotInitializedError
	}
	return j.bleveIndex.Similar(ctx, makeSearchDocument(c, i), searchResultCount)
}

// makeSearchDocument creates a search document from a collection item.
func makeSearchDocument(c *Collection, i Item) search.Document {
	// Collect people involved in the item
	people := make([]string, 0, len(i.Actors())+len(i.Directors())+len(i.Writers()))
	for actorName := range i.Actors() {
		people = append(people, strings.ToLower(actorName))
	}
	for _, director := range i.Directors() {
		people = append(people, strings.ToLower(director))
	}
	for _, writer := range i.Writers() {
		people = append(people, strings.ToLower(writer))
	}

	// Strings need to be lowercase as all search matching is done in lower case.
	doc := search.Document{
		ID:        i.ID(),
		ParentID:  c.ID,
		Name:      strings.ToLower(i.Title()),
		NameExact: strings.ToLower(i.Title()),
		SortName:  strings.ToLower(i.SortName()),
		Overview:  strings.ToLower(i.Plot()),
		Genres:    i.Genres(),
		People:    people,
	}
	// log.Printf("makeSearchDocument: item %s (%s), type: %s, name: %s\n", i.ID(), c.ID, t, name)
	return doc
}
