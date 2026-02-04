// CollectionRepo provides access to content collections such as movies, shows, etc.
// It is responsible for managing collections, adding new ones, and updating them
// with the latest content from the filesystem.
package collection

import (
	"context"
	"errors"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/erikbos/jellofin-server/collection/search"
	"github.com/erikbos/jellofin-server/database"
	"github.com/erikbos/jellofin-server/idhash"
)

// CollectionRepo is a repository holding content collections.
type CollectionRepo struct {
	collections Collections
	repo        database.Repository
	bleveIndex  *search.Search
}

type Options struct {
	Collections []Collection
	Repo        database.Repository
}

// New creates a new CollectionRepo with the provided options.
func New(options *Options) *CollectionRepo {
	c := &CollectionRepo{
		collections: options.Collections,
		repo:        options.Repo,
	}
	return c
}

// AddCollection adds a new content collection to the repository.
func (cr *CollectionRepo) AddCollection(name string, ID string,
	collectiontype string, directory string, baseUrl string, hlsServer string) {

	var ct CollectionType
	switch collectiontype {
	case "movies":
		ct = CollectionTypeMovies
	case "shows":
		ct = CollectionTypeShows
	default:
		log.Fatalf("Unknown collection type %s, skipping", collectiontype)
		return
	}

	c := Collection{
		Name:      name,
		ID:        ID,
		Type:      ct,
		Directory: directory,
		// BaseUrl:   baseUrl,
		HlsServer: hlsServer,
	}
	// If no collection ID is provided, generate one based upon the name.
	if c.ID == "" {
		c.ID = idhash.IdHash(c.Name)
	}

	log.Printf("Adding collection %s, id: %s, type: %s, directory: %s\n", c.Name, c.ID, c.Type, c.Directory)

	cr.collections = append(cr.collections, c)
}

// Init starts scanning the repository for contents for the first time.
func (cr *CollectionRepo) Init() {
	log.Printf("Initializing collections..")
	// scan all collections without delay
	cr.updateCollections(0)
	// Build search index
	cr.BuildSearchIndex(context.Background())
}

// Background keeps scanning the repository for content changes continously.
func (cr *CollectionRepo) Background(ctx context.Context) {
	for {
		// scan all collections with delay
		cr.updateCollections(1500 * time.Millisecond)
		// Rebuild search index to ensure any new items are included
		cr.BuildSearchIndex(ctx)
	}
}

// updateCollections updates the collections with the latest content from file system.
// - ScanInterval can be set as wait time between loading details of individual items.
// This can be useful to avoid overloading the filesystem with too many requests.
func (cr *CollectionRepo) updateCollections(scanInterval time.Duration) {
	for i := range cr.collections {
		c := &(cr.collections[i])
		switch c.Type {
		case CollectionTypeMovies:
			cr.buildMovies(c, scanInterval)
		case CollectionTypeShows:
			cr.buildShows(c, scanInterval)
		default:
			log.Printf("Unknown collection type %s, skipping", c.Type)
		}
	}
}

// GetCollections returns all collections in the repository.
func (cr *CollectionRepo) GetCollections() Collections {
	return cr.collections
}

// GetCollection returns a collection by its ID.
func (cr *CollectionRepo) GetCollection(collectionID string) (c *Collection) {
	for n := range cr.collections {
		if cr.collections[n].ID == collectionID {
			c = &(cr.collections[n])
			return
		}
	}
	return
}

// GetItem returns an item in a collection by its ID or name.
func (cr *CollectionRepo) GetItem(collectionID string, itemName string) (i Item) {
	c := cr.GetCollection(collectionID)
	if c == nil {
		return
	}
	for _, n := range c.Items {
		if n.Name() == itemName || n.ID() == itemName {
			return n
		}
		// If item is a show, also search in seasons and episodes
		switch v := n.(type) {
		case *Show:
			for _, s := range v.Seasons {
				if s.ID() == itemName {
					return &s
				}
				for _, e := range s.Episodes {
					if e.ID() == itemName {
						return &e
					}
				}
			}
		}
	}
	return
}

// GetItemByID returns an item in a collection by its ID.
func (cr *CollectionRepo) GetItemByID(itemID string) (*Collection, Item) {
	for _, c := range cr.collections {
		if i := cr.GetItem(c.ID, itemID); i != nil {
			return &c, i
		}
	}
	return nil, nil
}

// GetSeasonByID returns a season in a collection by its ID.
func (cr *CollectionRepo) GetSeasonByID(saesonID string) (*Collection, *Show, *Season) {
	// fixme: wooho O(n^^3) "just temporarily.."
	for _, c := range cr.collections {
		for _, i := range c.Items {
			switch v := i.(type) {
			case *Show:
				for _, s := range v.Seasons {
					if s.id == saesonID {
						return &c, v, &s
					}
				}
			}
		}
	}
	return nil, nil, nil
}

// GetEpisodeByID returns an episode in a collection by its ID.
func (cr *CollectionRepo) GetEpisodeByID(episodeID string) (*Collection, *Show, *Season, *Episode) {
	// fixme: wooho O(n^^4) "just temporarily.."
	for _, c := range cr.collections {
		for _, i := range c.Items {
			switch v := i.(type) {
			case *Show:
				for _, s := range v.Seasons {
					for _, e := range s.Episodes {
						if e.id == episodeID {
							return &c, v, &s, &e
						}
					}
				}
			}
		}
	}
	return nil, nil, nil, nil
}

// NextUp returns the nextup episodes in the collection based upon list of watched episodes
func (cr *CollectionRepo) NextUp(watchedEpisodeIDs []string) (nextUpEpisodeIDs []string, e error) {

	type ShowEntry struct {
		show          *Show
		seasonNumber  int
		episodeNumber int
		seasonIdx     int
		epIdx         int
	}
	showMap := make(map[string]ShowEntry)

	for _, episodeID := range watchedEpisodeIDs {

		c, show, season, episode := cr.GetEpisodeByID(episodeID)
		if c == nil || show == nil || season == nil || episode == nil {
			continue
		}

		// NextUp skips everything apart from shows
		if c.Type != CollectionTypeShows {
			continue
		}

		log.Printf("NextUp: %s(%s) %s, %d-%d\n", show.name, show.id, episode.id, episode.SeasonNo, episode.EpisodeNo)

		// Find season and episode index
		seasonIdx, epIdx := -1, -1
		// seasonIdx = season.SeasonNo - 1
		// epIdx = episode.EpisodeNo - 1
		for si, s := range show.Seasons {
			if s.id == season.id {
				seasonIdx = si
				for ei, e := range s.Episodes {
					if e.id == episode.id {
						epIdx = ei
						break
					}
				}
				break
			}
		}
		if seasonIdx == -1 || epIdx == -1 {
			continue
		}

		entry, exists := showMap[show.id]
		// No entries for this show, add it
		if !exists ||
			// watched item is in next season
			season.seasonno > entry.seasonNumber ||
			// watched item is in same season but next episode
			(season.seasonno == entry.seasonNumber && episode.EpisodeNo > entry.episodeNumber) {
			showMap[show.id] = ShowEntry{
				show:          show,
				seasonNumber:  season.seasonno,
				episodeNumber: episode.EpisodeNo,
				seasonIdx:     seasonIdx,
				epIdx:         epIdx,
			}
		}
	}

	log.Printf("NextUp: showMap: %+v\n", showMap)

	nextUpEpisodeIDs = make([]string, 0)
	for _, entry := range showMap {
		item := entry.show
		seasonIdx := entry.seasonIdx
		epIdx := entry.epIdx

		if seasonIdx < len(item.Seasons) {
			season := &item.Seasons[seasonIdx]
			if epIdx+1 < len(season.Episodes) {
				log.Printf("Adding: in same season %s(%s) %s, %d-%d\n", item.name, item.id, season.Episodes[epIdx+1].id, seasonIdx, epIdx+1)
				// Try next episode in same season
				nextUpEpisodeIDs = append(nextUpEpisodeIDs, season.Episodes[epIdx+1].id)
				continue
			}
			// Try first episode in next season
			if seasonIdx+1 < len(item.Seasons) && len(item.Seasons[seasonIdx+1].Episodes) > 0 {
				log.Printf("Adding: in next season %s(%s) %s, %d-%d\n", item.name, item.id, item.Seasons[seasonIdx+1].Episodes[0].id, seasonIdx+1, 0)
				nextUpEpisodeIDs = append(nextUpEpisodeIDs, item.Seasons[seasonIdx+1].Episodes[0].id)
			}
		}
	}

	return nextUpEpisodeIDs, nil
}

// Details returns collection details such as genres, tags, ratings, etc.
func (c *CollectionRepo) Details() CollectionDetails {
	var movieCount, showCount, episodeCount int
	genres := make([]string, 0)
	studios := make([]string, 0)
	tags := make([]string, 0)
	official := make([]string, 0)
	years := make([]int, 0)

	for _, collection := range c.collections {
		details := collection.Details()

		movieCount += details.MovieCount
		showCount += details.ShowCount
		episodeCount += details.EpisodeCount

		for _, g := range details.Genres {
			if !slices.Contains(genres, g) {
				genres = append(genres, g)
			}
		}
		for _, s := range details.Studios {
			if !slices.Contains(studios, s) {
				studios = append(studios, s)
			}
		}
		for _, t := range details.Tags {
			if !slices.Contains(tags, t) {
				tags = append(tags, t)
			}
		}
		for _, r := range details.OfficialRatings {
			if !slices.Contains(official, r) {
				official = append(official, r)
			}
		}
	}

	details := CollectionDetails{
		MovieCount:      movieCount,
		ShowCount:       showCount,
		EpisodeCount:    episodeCount,
		Genres:          genres,
		Tags:            tags,
		OfficialRatings: official,
		Years:           years,
	}
	return details
}

// GenreItemCount returns number of items per genre.
func (c *CollectionRepo) GenreItemCount() map[string]int {
	genreCount := make(map[string]int)
	for _, collection := range c.collections {
		for _, i := range collection.Items {
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
	}
	return genreCount
}

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
