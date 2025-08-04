// CollectionRepo provides access to content collections such as movies, shows, etc.
// It is responsible for managing collections, adding new ones, and updating them
// with the latest content from the filesystem.
package collection

import (
	"log"
	"slices"
	"time"

	"github.com/erikbos/jellofin-server/database"
	"github.com/erikbos/jellofin-server/idhash"
)

// CollectionRepo is a repository holding content collections.
type CollectionRepo struct {
	collections Collections
	db          *database.DatabaseRepo
}

type Options struct {
	Collections []Collection
	Db          *database.DatabaseRepo
}

// New creates a new CollectionRepo with the provided options.
func New(options *Options) *CollectionRepo {
	c := &CollectionRepo{
		collections: options.Collections,
		db:          options.Db,
	}
	return c
}

// AddCollection adds a new content collection to the repository.
func (cr *CollectionRepo) AddCollection(name string, ID string,
	collType string, directory string, baseUrl string, hlsServer string) {

	c := Collection{
		Name:      name,
		ID:        ID,
		Type:      collType,
		Directory: directory,
		// BaseUrl:   baseUrl,
		HlsServer: hlsServer,
	}
	// If no collection ID is provided, generate one based upon the name.
	if c.ID == "" {
		c.ID = idhash.IdHash(c.Name)
	}

	log.Printf("Adding collection %s (%s), type: %s, directory: %s\n", c.Name, c.ID, c.Type, c.Directory)

	cr.collections = append(cr.collections, c)
}

// Init starts scanning the repository for contents for the first time.
func (cr *CollectionRepo) Init() {
	// scan all collections without delay
	cr.updateCollections(0)
}

// Background keeps scanning the repository for content changes continously.
func (cr *CollectionRepo) Background() {
	for {
		// scan all collections with delay
		cr.updateCollections(500 * time.Millisecond)
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

// GetCollectionItems returns all items in a collection by its ID.
func (cr *CollectionRepo) GetCollectionItems(colllectionID string) []Item {
	items := make([]Item, 0)

	for _, c := range cr.collections {
		// Skip if we are searching in one particular collection?
		if c.ID != colllectionID {
			continue
		}
		for _, i := range c.Items {
			items = append(items, *i)
		}
	}
	return items
}

// GetItem returns an item in a collection by its ID or name.
func (cr *CollectionRepo) GetItem(collectionID string, itemName string) (i *Item) {
	c := cr.GetCollection(collectionID)
	if c == nil {
		return
	}
	for _, n := range c.Items {
		if n.Name == itemName || n.ID == itemName {
			i = n
			return
		}
	}
	return
}

// GetItemByID returns an item in a collection by its ID.
func (cr *CollectionRepo) GetItemByID(itemID string) (c *Collection, i *Item) {
	for _, c := range cr.collections {
		if i = cr.GetItem(c.ID, itemID); i != nil {
			return &c, i
		}
	}
	return nil, nil
}

// GetSeasonByID returns a season in a collection by its ID.
func (cr *CollectionRepo) GetSeasonByID(saesonID string) (*Collection, *Item, *Season) {
	// fixme: wooho O(n^^3) "just temporarily.."
	for _, c := range cr.collections {
		for _, i := range c.Items {
			for _, s := range i.Seasons {
				if s.ID == saesonID {
					return &c, i, &s
				}
			}
		}
	}
	return nil, nil, nil
}

// GetEpisodeByID returns an episode in a collection by its ID.
func (cr *CollectionRepo) GetEpisodeByID(episodeID string) (*Collection, *Item, *Season, *Episode) {
	// fixme: wooho O(n^^4) "just temporarily.."
	for _, c := range cr.collections {
		for _, i := range c.Items {
			for _, s := range i.Seasons {
				for _, e := range s.Episodes {
					if e.ID == episodeID {
						return &c, i, &s, &e
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
		show          *Item
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

		log.Printf("NextUp: %s(%s) %s, %d-%d\n", show.Name, show.ID, episode.ID, episode.SeasonNo, episode.EpisodeNo)

		// Find season and episode index
		seasonIdx, epIdx := -1, -1
		// seasonIdx = season.SeasonNo - 1
		// epIdx = episode.EpisodeNo - 1
		for si, s := range show.Seasons {
			if s.ID == season.ID {
				seasonIdx = si
				for ei, e := range s.Episodes {
					if e.ID == episode.ID {
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

		entry, exists := showMap[show.ID]
		// No entries for this show, add it
		if !exists ||
			// watched item is in next season
			season.SeasonNo > entry.seasonNumber ||
			// watched item is in same season but next episode
			(season.SeasonNo == entry.seasonNumber && episode.EpisodeNo > entry.episodeNumber) {
			showMap[show.ID] = ShowEntry{
				show:          show,
				seasonNumber:  season.SeasonNo,
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
				log.Printf("Adding: in same season %s(%s) %s, %d-%d\n", item.Name, item.ID, season.Episodes[epIdx+1].ID, seasonIdx, epIdx+1)
				// Try next episode in same season
				nextUpEpisodeIDs = append(nextUpEpisodeIDs, season.Episodes[epIdx+1].ID)
				continue
			}
			// Try first episode in next season
			if seasonIdx+1 < len(item.Seasons) && len(item.Seasons[seasonIdx+1].Episodes) > 0 {
				log.Printf("Adding: in next season %s(%s) %s, %d-%d\n", item.Name, item.ID, item.Seasons[seasonIdx+1].Episodes[0].ID, seasonIdx+1, 0)
				nextUpEpisodeIDs = append(nextUpEpisodeIDs, item.Seasons[seasonIdx+1].Episodes[0].ID)
			}
		}
	}

	return nextUpEpisodeIDs, nil
}

// Details returns collection details such as genres, tags, ratings, etc.
func (c *CollectionRepo) Details() CollectionDetails {
	genres := make([]string, 0)
	tags := make([]string, 0)
	official := make([]string, 0)
	years := make([]int, 0)

	for _, collection := range c.collections {
		for _, i := range collection.Items {
			for _, g := range i.Genres {
				g := normalizeGenre(g)
				if !slices.Contains(genres, g) {
					genres = append(genres, g)
				}
			}
			if i.OfficialRating != "" && !slices.Contains(official, i.OfficialRating) {
				official = append(official, i.OfficialRating)
			}
			if i.Year != 0 && !slices.Contains(years, i.Year) {
				years = append(years, i.Year)
			}
		}
	}

	details := CollectionDetails{
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
			for _, g := range i.Genres {
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
