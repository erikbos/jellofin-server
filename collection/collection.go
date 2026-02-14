package collection

import "slices"

type Collection struct {
	// Unique identifier for the collection. Hash of the collection name, or taken from configfile.
	ID string
	// Name of the collection, .e.g., "My Favorite Movies"
	Name string
	// Type of the collection, e.g., "movies", "shows"
	Type CollectionType
	// Items in the collection, could be type movies or shows
	Items []Item
	// Directory where the collection is stored
	Directory string
	// BaseUrl   string
	// HLS server URL for streaming content
	HlsServer string
}

type CollectionType string

const (
	CollectionTypeMovies CollectionType = "movies"
	CollectionTypeShows  CollectionType = "shows"
)

type Collections []Collection

func (c *Collection) GetHlsServer() string {
	return c.HlsServer
}

func (c *Collection) GetDataDir() string {
	return c.Directory
}

// Return list of genres from collection.
func (c *Collection) Genres() []string {
	var collectionGenres []string
	for _, i := range c.Items {
		for _, genre := range i.Genres() {
			if !slices.Contains(collectionGenres, genre) {
				collectionGenres = append(collectionGenres, genre)
			}
		}
	}
	return collectionGenres
}
