package collection

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"

	"github.com/miquels/notflix-server/database"
	"github.com/miquels/notflix-server/nfo"
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
	nfoPath    string
	nfoTime    int64
	Nfo        *nfo.Nfo
	Banner     string
	Fanart     string
	Folder     string
	Poster     string
	Rating     float32
	Votes      int
	Genre      []string
	Year       int

	// movie
	Video   string
	Thumb   string
	SrtSubs []Subs
	VttSubs []Subs

	// show
	SeasonAllBanner string
	SeasonAllFanart string
	SeasonAllPoster string
	Seasons         []Season
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
	Nfo       *nfo.Nfo
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

func (cr *CollectionRepo) GetItemByID(itemId string) (c *Collection, i *Item) {
	for _, c := range cr.collections {
		if i = cr.GetItem(c.Name_, itemId); i != nil {
			return &c, i
		}
	}
	return nil, nil
}

func (cr *CollectionRepo) GetSeasonByID(saesonId string) (*Collection, *Item, *Season) {
	// fixme: wooho O(n^^3) "just temporarily.."
	for _, c := range cr.collections {
		for _, i := range c.Items {
			for _, s := range i.Seasons {
				if s.ID == saesonId {
					return &c, i, &s
				}
			}
		}
	}
	return nil, nil, nil
}

func (cr *CollectionRepo) GetEpisodeByID(episodeId string) (*Collection, *Item, *Season, *Episode) {
	// fixme: wooho O(n^^4) "just temporarily.."
	for _, c := range cr.collections {
		for _, i := range c.Items {
			for _, s := range i.Seasons {
				for _, e := range s.Episodes {
					if e.ID == episodeId {
						return &c, i, &s, &e
					}

				}
			}
		}
	}
	return nil, nil, nil, nil
}

func (c *Collection) GetHlsServer() string {
	return c.HlsServer
}

func (c *Collection) GetDataDir() string {
	return c.Directory
}

// Details returns collection details such as genres, tags, ratings, etc.
func (c *CollectionRepo) Details() CollectionDetails {
	genre := make(map[string]bool)
	tags := make(map[string]bool)
	ratings := make(map[string]bool)
	years := make(map[int]bool)

	for _, c := range c.collections {
		for _, i := range c.Items {
			// Fixme: i.Genre is not populated
			// if i.Genre != nil {
			// 	for _, g := range i.Genre {
			// 		genre[g] = true
			// 	}
			// }
			// if i.Rating != 0 {
			// 	ratings[i.Rating] = true
			// }
			if i.Year != 0 {
				years[i.Year] = true
			}
		}
	}

	details := CollectionDetails{
		Genres:          returnStringArray(genre),
		Tags:            returnStringArray(tags),
		OfficialRatings: returnStringArray(ratings),
		Years:           returnIntArray(years),
	}
	return details
}

// Details returns collection details such as genres, tags, ratings, etc.
func (c *Collection) Details() CollectionDetails {
	genre := make(map[string]bool)
	tags := make(map[string]bool)
	ratings := make(map[string]bool)
	years := make(map[int]bool)

	for _, i := range c.Items {
		// Fixme: i.Genre is not populated
		// if i.Genre != nil {
		// 	for _, g := range i.Genre {
		// 		genre[g] = true
		// 	}
		// }
		// if i.Rating != 0 {
		// 	ratings[i.Rating] = true
		// }
		if i.Year != 0 {
			years[i.Year] = true
		}
	}

	details := CollectionDetails{
		Genres:          returnStringArray(genre),
		Tags:            returnStringArray(tags),
		OfficialRatings: returnStringArray(ratings),
		Years:           returnIntArray(years),
	}
	return details
}

func returnStringArray(m map[string]bool) []string {
	result := []string{}
	for k := range m {
		result = append(result, k)
	}
	return result
}

func returnIntArray(m map[int]bool) []int {
	result := []int{}
	for k := range m {
		result = append(result, k)
	}
	sort.Ints(result)
	return result
}

// LoadNfo loads the NFO file for the item if not loaded already
func (i *Item) LoadNfo() {
	loadNFO(&i.Nfo, i.nfoPath)
}

// LoadNfo loads the NFO file for the episode if not loaded already
func (e *Episode) LoadNfo() {
	loadNFO(&e.Nfo, e.nfoPath)
}
