package collection

import (
	"slices"
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

// CollectionDetails contains aggregate details about a collection.
type CollectionDetails struct {
	// Number of movies.
	MovieCount int
	// Number of shows.
	ShowCount int
	// Number of episodes.
	EpisodeCount int
	// List of genres.
	Genres []string
	// List of studios.
	Studios []string
	// List of tags.
	Tags []string
	// List of official ratings.
	OfficialRatings []string
	// List of years.
	Years []int
}

// Details returns collection details such as genres, tags, ratings, etc.
func (c *Collection) Details() CollectionDetails {
	var movieCount, showCount, episodeCount int
	genres := make([]string, 0)
	studios := make([]string, 0)
	tags := make([]string, 0)
	official := make([]string, 0)
	years := make([]int, 0)

	for _, i := range c.Items {
		switch t := i.(type) {
		case *Movie:
			movieCount++
		case *Show:
			showCount++
			for _, e := range t.Seasons {
				episodeCount += len(e.Episodes)
			}
		}

		for _, g := range i.Genres() {
			if !slices.Contains(genres, g) {
				genres = append(genres, g)
			}
		}

		for _, s := range i.Studios() {
			if !slices.Contains(studios, s) {
				studios = append(studios, s)
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

	details := CollectionDetails{
		MovieCount:      movieCount,
		ShowCount:       showCount,
		EpisodeCount:    episodeCount,
		Genres:          genres,
		Studios:         studios,
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
