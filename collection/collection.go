package collection

import (
	"slices"

	"github.com/erikbos/jellofin-server/collection/metadata"
)

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

// CollectionDetails contains details about a collection.
type CollectionDetails struct {
	Genres          []string
	Tags            []string
	OfficialRatings []string
	Years           []int
}

// Details returns collection details such as genres, tags, ratings, etc.
func (c *Collection) Details() CollectionDetails {
	genres := make([]string, 0)
	tags := make([]string, 0)
	official := make([]string, 0)
	years := make([]int, 0)

	for _, i := range c.Items {
		itemGenres := i.Genres()

		for _, g := range itemGenres {
			g := metadata.NormalizeGenre(g)
			if !slices.Contains(itemGenres, g) {
				itemGenres = append(itemGenres, g)
			}
		}

		itemOfficialRating := i.OfficialRating()
		if itemOfficialRating != "" && !slices.Contains(official, itemOfficialRating) {
			official = append(official, itemOfficialRating)
		}

		itemYear := i.Year()
		if itemYear != 0 && !slices.Contains(years, itemYear) {
			years = append(years, itemYear)
		}
	}

	slices.Sort(years)

	details := CollectionDetails{
		Genres:          genres,
		Tags:            tags,
		OfficialRatings: official,
		Years:           years,
	}
	return details
}

// GenreCount returns number of items per genre of a collection.
func (c *Collection) GenreCount() map[string]int {
	genreCount := make(map[string]int)
	for _, i := range c.Items {
		for _, g := range i.Genres() {
			if g == "" {
				continue
			}
			if _, found := genreCount[g]; !found {
				genreCount[g] = 1
			} else {
				genreCount[g] += 1
			}
		}
	}
	return genreCount
}
