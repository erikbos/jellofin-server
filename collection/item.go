package collection

import (
	"strings"
	"unicode"
)

// An 'item' can be a movie, a tv-show, a folder, etc.
type Item struct {
	// ID is the unique identifier for the item. Typically Idhash() of name.
	ID string
	// Name is the name of the item, e.g. "Casablanca (1949)"
	Name string
	// SortName is used to sort on.
	SortName string
	// Type indicates item type, e.g., "movie" or "show".
	Type string
	// Path is the directory to the item, relative to collection root.
	Path string
	// BaseUrl is the base URL for accessing the item.
	BaseUrl string
	// FirstVideo is the timestamp of the first video in the item.
	FirstVideo int64
	// LastVideo is the timestamp of the last video in the item.
	LastVideo int64
	// Banner is the item's banner image, often "banner.jpg", TV shows only.
	Banner string
	// Fanart is this item's fanart image, often "fanart.jpg"
	Fanart string
	// Folder is this item's folder image, often "folder.jpg"
	Folder string
	// Posten is this item's poster image, often "poster.jpg"
	Poster string
	// Logo is this item's transparent logo, often "clearlogo.png", TV shows only.
	Logo string
	// Filename of the video file, e.g. "casablanca.mp4"
	FileName string
	// FileSize is the size of the video file in bytes.
	FileSize int64
	// TODO: Thumb is unused
	Thumb   string
	SrtSubs Subtitles
	VttSubs Subtitles

	// TODO: perhaps optional, do we need these? Can we replace with Banner,Fanart,Poster?
	SeasonAllBanner string
	SeasonAllFanart string
	SeasonAllPoster string
	Seasons         Seasons

	// nfoPath is the full path to the NFO file, e.g. "/mnt/media/casablanca.nfo"
	nfoPath string
	// TODO: nfoTime is unused.
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
	// ID is the unique identifier of the season.
	ID string
	// SeasonNo is the season number, e.g., 1, 2, etc. 0 is used for specials.
	SeasonNo int
	// Banner is the path to the season banner image.
	Banner string
	// Fanart is the path to the season fanart image.
	Fanart string
	// Poster is the path to the season poster image, e.g. "season01-poster.jpg"
	Poster string
	// Episodes contains the episodes in this season.
	Episodes Episodes
}

type Seasons []Season

func (s Seasons) Len() int {
	return len(s)
}

func (s Seasons) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s Seasons) Less(i, j int) bool {
	return s[i].SeasonNo < s[j].SeasonNo
}

type Episode struct {
	// ID is the unique identifier of the item. Typically Idhash() of name.
	ID string
	// Name is the human-readable name of the episode.
	Name string
	// SortName is the name of the item when sorting is applied.
	SortName string
	// SeasonNo is the season number, e.g., 1, 2, etc. 0 is used for specials.
	SeasonNo int
	// EpisodeNo is the episode number within the season, e.g., 1, 2, etc.
	EpisodeNo int
	// Double indicates if this is a double episode, e.g., 1-2.
	Double bool
	// BaseName is the base name of the episode, e.g., "casablanca.s01e01"
	BaseName string
	// nfoPath is the full path to the NFO file, e.g. "/mnt/media/casablanca.nfo"
	nfoPath string
	// TODO: nfoTime is unused.
	nfoTime int64
	// VideoTS is the timestamp of the episode.
	VideoTS int64
	Nfo     *Nfo
	// FileName is the filename relative to show item directory, e.g. "S01/casablanca.s01e01.mp4"
	FileName string
	// FileSize is the size of the video file in bytes.
	FileSize int64
	// Thumb is the thumbname image relative to show item directory, e.g. "S01/casablanca.s01e01-thumb.jpg"
	Thumb   string
	SrtSubs Subtitles
	VttSubs Subtitles
}

type Episodes []Episode

func (e Episodes) Len() int {
	return len(e)
}

func (e Episodes) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func (e Episodes) Less(i, j int) bool {
	return e[i].EpisodeNo < e[j].EpisodeNo
}

type Subs struct {
	Lang string
	Path string
}

type Subtitles []Subs

// type PathString string

// func (p PathString) MarshalText() (text []byte, err error) {
// 	u := url.URL{Path: string(p)}
// 	text = []byte(u.EscapedPath())
// 	return
// }

// func (p PathString) String() string {
// 	return string(p)
// }

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

// makeSortName returns a name suitable for sorting.
func makeSortName(name string) string {
	// Start with lowercasing and trimming whitespace.
	title := strings.ToLower(strings.TrimSpace(name))

	// Remove leading articles.
	for _, prefix := range []string{"the ", "a ", "an "} {
		if strings.HasPrefix(title, prefix) {
			title = strings.TrimSpace(title[len(prefix):])
			break
		}
	}

	// Remove whitespace and punctuation.
	title = strings.TrimLeftFunc(title, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	})

	return title
}
