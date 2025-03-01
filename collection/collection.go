package collection

import (
	"net/url"
	"strconv"

	"github.com/miquels/notflix-server/nfo"
)

var config CollectionRepo

type CollectionRepo struct {
	collections []Collection
}

func New(c []Collection) {
	config.collections = c
}

type Collection struct {
	SourceId  int     `json:"id"`
	Name_     string  `json:"name"`
	Type      string  `json:"type"`
	Items     []*Item `json:"items,omitempty"`
	Directory string  `json:"-"`
	BaseUrl   string  `json:"-"`
	HlsServer string  `json:"-"`
}

// An 'item' can be a movie, a tv-show, a folder, etc.
type Item struct {
	// generic
	Id          string   `json:"id"`
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	BaseUrl     string   `json:"baseurl"`
	Type        string   `json:"type"`
	FirstVideo  int64    `json:"firstvideo,omitempty"`
	LastVideo   int64    `json:"lastvideo,omitempty"`
	SortName    string   `json:"sortName,omitempty"`
	NfoPath     string   `json:"-"`
	NfoTime     int64    `json:"-"`
	Nfo         *nfo.Nfo `json:"nfo,omitempty"`
	Banner      string   `json:"banner,omitempty"`
	Fanart      string   `json:"fanart,omitempty"`
	Folder      string   `json:"folder,omitempty"`
	Poster      string   `json:"poster,omitempty"`
	Rating      float32  `json:"rating,omitempty"`
	Votes       int      `json:"votes,omitempty"`
	Genre       []string `json:"genre,omitempty"`
	Genrestring string   `json:"-"`
	Year        int      `json:"year,omitempty"`

	// movie
	Video   string `json:"video,omitempty"`
	Thumb   string `json:"thumb,omitempty"`
	SrtSubs []Subs `json:"srtsubs,omitempty"`
	VttSubs []Subs `json:"vttsubs,omitempty"`

	// show
	SeasonAllBanner string   `json:"seasonAllBanner,omitempty"`
	SeasonAllFanart string   `json:"seasonAllFanart,omitempty"`
	SeasonAllPoster string   `json:"seasonAllPoster,omitempty"`
	Seasons         []Season `json:"seasons,omitempty"`
}

type Season struct {
	Id       string    `json:"id"`
	SeasonNo int       `json:"seasonno"`
	Banner   string    `json:"banner,omitempty"`
	Fanart   string    `json:"fanart,omitempty"`
	Poster   string    `json:"poster,omitempty"`
	Episodes []Episode `json:"episodes,omitempty"`
}

type Episode struct {
	Id        string   `json:"id"`
	Name      string   `json:"name"`
	SeasonNo  int      `json:"seasonno"`
	EpisodeNo int      `json:"episodeno"`
	Double    bool     `json:"double,omitempty"`
	SortName  string   `json:"sortName,omitempty"`
	BaseName  string   `json:"-"`
	NfoPath   string   `json:"-"`
	NfoTime   int64    `json:"-"`
	VideoTS   int64    `json:"-"`
	Nfo       *nfo.Nfo `json:"nfo,omitempty"`
	Video     string   `json:"video"`
	Thumb     string   `json:"thumb,omitempty"`
	SrtSubs   []Subs   `json:"srtsubs,omitempty"`
	VttSubs   []Subs   `json:"vttsubs,omitempty"`
}

type Subs struct {
	Lang string `json:"lang"`
	Path string `json:"path"`
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

const (
	CollectionMovies = "movies"
	CollectionShows  = "shows"
)

// func updateCollections(pace int) {
// 	id := 1
// 	for i := range config.Collections {
// 		c := &(config.Collections[i])
// 		c.SourceId = id
// 		c.BaseUrl = fmt.Sprintf("/data/%d", id)
// 		switch c.Type {
// 		case CollectionMovies:
// 			buildMovies(c, pace)
// 		case CollectionShows:
// 			buildShows(c, pace)
// 		}
// 		id++
// 	}
// }

// func initCollections() {
// 	updateCollections(0)
// }

func GetCollection(collName string) (c *Collection) {
	sourceId := -1
	if n, err := strconv.Atoi(collName); err == nil {
		sourceId = n
	}
	for n := range config.collections {
		if config.collections[n].Name_ == collName ||
			config.collections[n].SourceId == sourceId {
			c = &(config.collections[n])
			return
		}
	}
	return
}

func GetItem(collName string, itemName string) (i *Item) {
	c := GetCollection(collName)
	if c == nil {
		return
	}
	for _, n := range c.Items {
		if n.Name == itemName || n.Id == itemName {
			i = n
			return
		}
	}
	return
}

func GetItemByID(itemId string) (c *Collection, i *Item) {
	for _, c := range config.collections {
		if i = GetItem(c.Name_, itemId); i != nil {
			return &c, i
		}
	}
	return nil, nil
}

func GetSeasonByID(saesonId string) (*Collection, *Item, *Season) {
	// fixme: wooho O(n^^3) "just temporarily.."
	for _, c := range config.collections {
		for _, i := range c.Items {
			for _, s := range i.Seasons {
				if s.Id == saesonId {
					return &c, i, &s
				}
			}
		}
	}
	return nil, nil, nil
}

const itemprefix_separator = "_"

func GetEpisodeByID(episodeId string) (*Collection, *Item, *Season, *Episode) {
	// episodeId = strings.TrimPrefix(episodeId, itemprefix_separator)

	// fixme: wooho O(n^^4) "just temporarily.."
	for _, c := range config.collections {
		for _, i := range c.Items {
			for _, s := range i.Seasons {
				for _, e := range s.Episodes {
					if e.Id == episodeId {
						return &c, i, &s, &e
					}

				}
			}
		}
	}
	return nil, nil, nil, nil
}

func GetHlsServer(source string) (h string) {
	id, err := strconv.ParseInt(source, 10, 64)
	if err != nil {
		return
	}
	for n, c := range config.collections {
		if int64(c.SourceId) == id {
			h = config.collections[n].HlsServer
			return
		}
	}
	return
}

func GetDataDir(source string) (d string) {
	id, err := strconv.ParseInt(source, 10, 64)
	if err != nil {
		return
	}
	for n, c := range config.collections {
		if int64(c.SourceId) == id {
			d = config.collections[n].Directory
			return
		}
	}
	return
}
