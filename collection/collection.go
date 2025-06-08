package collection

import (
	"fmt"
	"log"
	"net/url"
	"slices"
	"strconv"

	"github.com/erikbos/jellofin/database"
)

type Options struct {
	Collections []Collection
	Db          *database.DatabaseRepo
}

type CollectionRepo struct {
	collections Collections
	db          *database.DatabaseRepo
}

func New(options *Options) *CollectionRepo {
	c := &CollectionRepo{
		collections: options.Collections,
		db:          options.Db,
	}
	return c
}

type Collection struct {
	ID        int
	Name_     string
	Type      string
	Items     []*Item
	Directory string
	BaseUrl   string
	HlsServer string
}

const (
	CollectionMovies = "movies"
	CollectionShows  = "shows"
)

// CollectionDetails contains details about a collection
type CollectionDetails struct {
	Genres          []string
	Tags            []string
	OfficialRatings []string
	Years           []int
}

type Collections []Collection

// An 'item' can be a movie, a tv-show, a folder, etc.
type Item struct {
	ID         string
	Name       string
	Path       string
	BaseUrl    string
	Type       string
	FirstVideo int64
	LastVideo  int64
	SortName   string
	Banner     string
	Fanart     string
	Folder     string
	Poster     string

	// movie
	Video   string
	Thumb   string
	SrtSubs []Subs
	VttSubs []Subs

	// show
	SeasonAllBanner string
	SeasonAllFanart string
	SeasonAllPoster string
	// Filename of transparent logo, e.g. "clearlogo.png"
	Logo    string
	Seasons []Season

	// Content metadata
	nfoPath string
	nfoTime int64
	Nfo     *Nfo

	Metadata *Metadata

	Genres         []string
	OfficialRating string
	Year           int
	Rating         float32
	Votes          int
}

type Metadata struct {
	Genre  []string
	Year   int
	Rating float32
	Votes  int
}

type Season struct {
	ID       string
	SeasonNo int
	Banner   string
	Fanart   string
	Poster   string
	Episodes []Episode
}

type Episode struct {
	ID        string
	Name      string
	SeasonNo  int
	EpisodeNo int
	Double    bool
	SortName  string
	BaseName  string
	nfoPath   string
	nfoTime   int64
	VideoTS   int64
	Nfo       *Nfo
	Video     string
	Thumb     string
	SrtSubs   []Subs
	VttSubs   []Subs
}

type Subs struct {
	Lang string
	Path string
}

type byItem []Item
type bySeason []Season
type byEpisode []Episode

type PathString string

func (e byEpisode) Len() int {
	return len(e)
}

func (e byEpisode) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func (e byEpisode) Less(i, j int) bool {
	return e[i].EpisodeNo < e[j].EpisodeNo
}

func (s bySeason) Len() int {
	return len(s)
}

func (s bySeason) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s bySeason) Less(i, j int) bool {
	return s[i].SeasonNo < s[j].SeasonNo
}

func (p PathString) MarshalText() (text []byte, err error) {
	u := url.URL{Path: string(p)}
	text = []byte(u.EscapedPath())
	return
}
func (p PathString) String() string {
	return string(p)
}

func (cr *CollectionRepo) updateCollections(pace int) {
	id := 1
	for i := range cr.collections {
		c := &(cr.collections[i])
		c.ID = id
		c.BaseUrl = fmt.Sprintf("/data/%d", id)
		switch c.Type {
		case CollectionMovies:
			cr.buildMovies(c, pace)
		case CollectionShows:
			cr.buildShows(c, pace)
		}
		id++
	}
}

// Init initalizes content collections
func (cr *CollectionRepo) Init() {
	cr.updateCollections(0)
}

// Background keeps scanning content collections for changes continously
func (cr *CollectionRepo) Background() {
	for {
		cr.updateCollections(1)
	}
}

func (cr *CollectionRepo) GetCollections() Collections {
	return cr.collections
}

func (cr *CollectionRepo) GetCollectionItems(collName string) []Item {
	items := make([]Item, 0)

	for _, c := range cr.collections {
		// Skip if we are searching in one particular collection?
		if collName != "" && collName != c.Name_ {
			continue
		}
		for _, i := range c.Items {
			items = append(items, *i)
		}
	}
	return items
}

func (cr *CollectionRepo) GetCollection(collName string) (c *Collection) {
	sourceId := -1
	if n, err := strconv.Atoi(collName); err == nil {
		sourceId = n
	}
	for n := range cr.collections {
		if cr.collections[n].Name_ == collName ||
			cr.collections[n].ID == sourceId {
			c = &(cr.collections[n])
			return
		}
	}
	return
}

func (cr *CollectionRepo) GetItem(collName string, itemName string) (i *Item) {
	c := cr.GetCollection(collName)
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

func (cr *CollectionRepo) GetItemByID(itemID string) (c *Collection, i *Item) {
	for _, c := range cr.collections {
		if i = cr.GetItem(c.Name_, itemID); i != nil {
			return &c, i
		}
	}
	return nil, nil
}

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

// Returns the nextup episodes in the collection based upon list of watched episodes
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
		if c.Type != CollectionShows {
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

func (c *Collection) GetHlsServer() string {
	return c.HlsServer
}

func (c *Collection) GetDataDir() string {
	return c.Directory
}

// Details returns collection details such as genres, tags, ratings, etc.
func (c *Collection) Details() CollectionDetails {
	genres := make([]string, 0)
	tags := make([]string, 0)
	official := make([]string, 0)
	years := make([]int, 0)

	for _, i := range c.Items {
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

	slices.Sort(years)

	details := CollectionDetails{
		Genres:          genres,
		Tags:            tags,
		OfficialRatings: official,
		Years:           years,
	}
	return details
}

// GenreCount returns number of items per genre.
func (c *Collection) GenreCount() map[string]int {
	genreCount := make(map[string]int)
	for _, i := range c.Items {
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
	return genreCount
}

// LoadNfo loads the NFO file for the item if not loaded already
func (i *Item) LoadNfo() {
	loadNFO(&i.Nfo, i.nfoPath)
	if i.Nfo != nil {
		i.Genres = i.Nfo.Genre
		i.OfficialRating = i.Nfo.Mpaa
		i.Year = i.Nfo.Year
	}
}

// LoadNfo loads the NFO file for the episode if not loaded already
func (e *Episode) LoadNfo() {
	loadNFO(&e.Nfo, e.nfoPath)
}
