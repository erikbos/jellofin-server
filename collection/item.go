package collection

import (
	"log"
	"strings"
	"unicode"
)

type Item interface {
	// ID returns the unique identifier for the item.
	ID() string
	// Name returns the name of the item, e.g. "Casablanca (1949)"
	Name() string
	// SortName returns the name used for sorting.
	SortName() string
	// Path returns the directory to the m, relative to collection root.
	Path() string
	// BaseUrl returns the base URL for accessing the item.
	BaseUrl() string
	// Banner returns the item's banner image, often "banner.jpg", TV shows only.
	Banner() string
	// Fanart returns this item's fanart image, often "fanart.jpg"
	Fanart() string
	// Folder returns this item's folder image, often "folder.jpg"
	Folder() string
	// Poster returns this item's poster image, often "poster.jpg"
	Poster() string
	// Logo returns this item's transparent logo, often "clearlogo.png", TV shows only.
	Logo() string
	// FileName returns the filename of the video file, e.g. "casablanca.mp4"
	FileName() string
	// FileSize returns the size of the video file in bytes.
	FileSize() int64

	Metadata

	// Duration returns the duration of the video in seconds.
	Duration() int
	// GetGenres returns the genres.
	GetGenres() []string
	GetYear() int
	GetRating() float32
	GetOfficialRating() string
	GetNfo() *Nfo
	LoadNfo()
}

type Metadata interface {
	// Duration returns the duration of the video in seconds.
	Duration() int
	// GetGenres returns the genres.
	GetGenres() []string
	GetYear() int
	GetRating() float32
	GetOfficialRating() string
	GetNfo() *Nfo
	LoadNfo()
}

// type Metadata struct {
// 	Genre  []string
// 	Year   int
// 	Rating float32
// 	Votes  int
// }

// Movie represents a movie m in a collection.
type Movie struct {
	// id is the unique identifier for the movie. Typically Idhash() of name.
	id string
	// name is the name of the movie, e.g. "Casablanca (1949)"
	name string
	// SortName is used to sort on.
	sortName string
	// path is the directory to the movie, relative to collection root.
	path string
	// baseUrl is the base URL for accessing the movie.
	baseUrl string
	// firstVideo is the timestamp of the first video in the movie.
	firstVideo int64
	// banner is the movie's banner image, often "banner.jpg", TV shows only.
	banner string
	// fanart is this movie's fanart image, often "fanart.jpg"
	fanart string
	// folder is this movie's folder image, often "folder.jpg"
	folder string
	// Posten is this movie's poster image, often "poster.jpg"
	poster string
	// logo is this movie's transparent logo, often "clearlogo.png", TV shows only.
	logo string
	// Filename, e.g. "casablanca.mp4"
	fileName string
	// fileSize is the size of the video file in bytes.
	fileSize int64
	SrtSubs  Subtitles
	VttSubs  Subtitles

	// nfoPath is the full path to the NFO file, e.g. "/mnt/media/casablanca.nfo"
	nfoPath string
	Nfo     *Nfo

	Metadata Metadata

	Genres         []string
	OfficialRating string
	Year           int
	Rating         float32
	Votes          int
}

func (m *Movie) ID() string       { return m.id }
func (m *Movie) Name() string     { return m.name }
func (m *Movie) SortName() string { return m.sortName }
func (m *Movie) Path() string     { return m.path }
func (m *Movie) BaseUrl() string  { return m.baseUrl }

// FirstVideo returns the timestamp of the first video in the item.
func (m *Movie) FirstVideo() int64 { return m.firstVideo }
func (m *Movie) Banner() string    { return m.banner }
func (m *Movie) Fanart() string    { return m.fanart }
func (m *Movie) Folder() string    { return m.folder }
func (m *Movie) Poster() string    { return m.poster }
func (m *Movie) Logo() string      { return m.logo }
func (m *Movie) FileName() string  { return m.fileName }
func (m *Movie) FilePath() string  { return m.path + "/" + m.fileName }
func (m *Movie) FileSize() int64   { return m.fileSize }
func (m *Movie) Duration() int {
	m.LoadNfo()
	if m.Nfo == nil {
		return 0
	}
	// Try primary field to get duration
	if m.Nfo.Runtime != 0 {
		return m.Nfo.Runtime * 60
	}
	// Fallback to NFO stream details if available
	if m.Nfo.FileInfo != nil &&
		m.Nfo.FileInfo.StreamDetails != nil &&
		m.Nfo.FileInfo.StreamDetails.Video != nil {
		return m.Nfo.FileInfo.StreamDetails.Video.DurationInSeconds
	}
	return 0
}

func (m *Movie) GetGenres() []string       { return m.Genres }
func (m *Movie) GetYear() int              { return m.Year }
func (m *Movie) GetRating() float32        { return m.Rating }
func (m *Movie) GetOfficialRating() string { return m.OfficialRating }
func (m *Movie) GetNfo() *Nfo              { return m.Nfo }

// LoadNfo loads the NFO file for the m if not loaded already
func (m *Movie) LoadNfo() {
	loadNFO(&m.Nfo, m.nfoPath)
	if m.Nfo != nil {
		m.Genres = m.Nfo.Genre
		m.OfficialRating = m.Nfo.Mpaa
		m.Year = m.Nfo.Year
	}
}

// Show represents a TV show with multiple seasons and episodes.
type Show struct {
	// id is the unique identifier for the m. Typically Idhash() of name.
	id string
	// name is the name of the m, e.g. "Casablanca (1949)"
	name string
	// sortName is used to sort on.
	sortName string
	// path is the directory to the show, relative to collection root. (e.g. Casablanca)
	path string
	// baseUrl is the base URL for accessing the m.
	baseUrl string
	// firstVideo is the timestamp of the first video in the m.
	firstVideo int64
	// lastVideo is the timestamp of the last video in the m.
	lastVideo int64
	// banner is the m's banner image, often "banner.jpg", TV shows only.
	banner string
	// fanart is this m's fanart image, often "fanart.jpg"
	fanart string
	// folder is this m's folder image, often "folder.jpg"
	folder string
	// posten is this m's poster image, often "poster.jpg"
	poster string
	// logo is this m's transparent logo, often "clearlogo.png", TV shows only.
	logo string
	// seasonAllBanner is the banner to be used in case we do not have a season-specific banner.
	seasonAllBanner string
	// seasonAllPoster to be used in case we do not have a season-specific poster.
	seasonAllPoster string
	// filename of the video file, e.g. "casablanca.mp4"
	fileName string
	// fileSize is the size of the video file in bytes.
	fileSize int64
	SrtSubs  Subtitles
	VttSubs  Subtitles
	// Seasons contains the seasons in this TV show.
	Seasons Seasons

	// nfoPath is the full path to the NFO file, e.g. "/mnt/media/casablanca.nfo"
	nfoPath string
	Nfo     *Nfo

	Metadata Metadata

	Genres         []string
	OfficialRating string
	Year           int
	Rating         float32
	Votes          int
}

func (s *Show) ID() string       { return s.id }
func (s *Show) Name() string     { return s.name }
func (s *Show) SortName() string { return s.sortName }
func (s *Show) Path() string     { return s.path }
func (s *Show) BaseUrl() string  { return s.baseUrl }

// FirstVideo returns the timestamp of the first video in the show.
func (s *Show) FirstVideo() int64 { return s.firstVideo }

// LastVideo returns the timestamp of the last video in the show.
func (s *Show) LastVideo() int64        { return s.lastVideo }
func (s *Show) Banner() string          { return s.banner }
func (s *Show) Fanart() string          { return s.fanart }
func (s *Show) Folder() string          { return s.folder }
func (s *Show) Poster() string          { return s.poster }
func (s *Show) Logo() string            { return s.logo }
func (s *Show) SeasonAllBanner() string { return s.seasonAllBanner }
func (s *Show) SeasonAllPoster() string { return s.seasonAllPoster }
func (s *Show) FileName() string        { return s.fileName }
func (s *Show) FileSize() int64         { return s.fileSize }
func (s *Show) Duration() int {
	var duration int
	for _, season := range s.Seasons {
		duration += season.Duration()
	}
	return duration
}
func (s *Show) GetGenres() []string       { return s.Genres }
func (s *Show) GetYear() int              { return s.Year }
func (s *Show) GetRating() float32        { return s.Rating }
func (s *Show) GetOfficialRating() string { return s.OfficialRating }
func (s *Show) GetNfo() *Nfo              { return s.Nfo }

// LoadNfo loads the NFO file for the m if not loaded already
func (s *Show) LoadNfo() {
	loadNFO(&s.Nfo, s.nfoPath)
	if s.Nfo != nil {
		s.Genres = s.Nfo.Genre
		s.OfficialRating = s.Nfo.Mpaa
		s.Year = s.Nfo.Year
	}
}

// Season represents a season of a TV show, containing multiple episodes.
type Season struct {
	// id is the unique identifier of the season.
	id string
	// name is the human-readable name of the season.
	name string
	// path is the directory to the show(!), relative to collection root. (e.g. Casablanca)
	path string
	// seasonno is the season seasonno, e.g., 1, 2, etc. 0 is used for specials.
	seasonno int
	// banner is the path to the season banner image.
	banner string
	// fanart is the path to the season fanart image.
	fanart string
	// poster is the path to the season poster image, e.g. "season01-poster.jpg"
	poster string
	// seasonAllBanner is the banner to be used in case we do not have a season-specific banner.
	seasonAllBanner string
	// seasonAllPoster to be used in case we do not have a season-specific poster.
	seasonAllPoster string
	// Episodes contains the episodes in this season.
	Episodes Episodes
}

func (season *Season) ID() string        { return season.id }
func (season *Season) Name() string      { return season.name }
func (season *Season) SortName() string  { return season.name }
func (season *Season) Path() string      { return season.path }
func (season *Season) BaseUrl() string   { return "" }
func (season *Season) Number() int       { return season.seasonno }
func (season *Season) FirstVideo() int64 { return 0 }
func (season *Season) LastVideo() int64  { return 0 }
func (season *Season) Banner() string    { return season.banner }
func (season *Season) Fanart() string    { return season.fanart }
func (season *Season) Folder() string    { return "" }
func (season *Season) Poster() string {
	if season.poster != "" {
		return season.poster
	}
	if season.seasonAllPoster != "" {
		return season.seasonAllPoster
	}
	return ""
}
func (season *Season) Logo() string     { return "" }
func (season *Season) FileName() string { return "" }
func (season *Season) FileSize() int64  { return 0 }
func (season *Season) Duration() int {
	var duration int
	for _, ep := range season.Episodes {
		duration += ep.Duration()
	}
	return duration
}
func (season *Season) GetGenres() []string       { return []string{} }
func (season *Season) GetYear() int              { return 0 }
func (season *Season) GetRating() float32        { return 0 }
func (season *Season) GetOfficialRating() string { return "" }
func (season *Season) GetNfo() *Nfo              { return nil }

// LoadNfo loads the NFO file for the m if not loaded already
func (season *Season) LoadNfo() {
}

type Seasons []Season

func (s Seasons) Len() int {
	return len(s)
}

func (s Seasons) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s Seasons) Less(i, j int) bool {
	return s[i].seasonno < s[j].seasonno
}

// Episode represents a single episode of a TV show.
type Episode struct {
	// id is the unique identifier of the m. Typically Idhash() of name.
	id string
	// Name is the human-readable name of the episode.
	name string
	// path is the directory of the show, relative to collection root. (e.g. Casablanca)
	path string
	// SortName is the name of the m when sorting is applied.
	sortName string
	// SeasonNo is the season number, e.g., 1, 2, etc. 0 is used for specials.
	SeasonNo int
	// EpisodeNo is the episode number within the season, e.g., 1, 2, etc.
	EpisodeNo int
	// Double indicates if this is a double episode, e.g., 1-2.
	Double bool
	// baseName is the base name of the episode, e.g., "casablanca.s01e01"
	baseName string
	// VideoTS is the timestamp of the episode.
	VideoTS int64
	// FileName is the filename relative to show directory, e.g. "S01/casablanca.s01e01.mp4"
	fileName string
	// fileSize is the size of the video file in bytes.
	fileSize int64
	// Thumb is the thumbname image relative to show directory, e.g. "S01/casablanca.s01e01-thumb.jpg"
	Thumb   string
	SrtSubs Subtitles
	VttSubs Subtitles
	// nfoPath is the full path to the NFO file, e.g. "/mnt/media/casablanca.nfo"
	nfoPath string
	Nfo     *Nfo
}

func (e *Episode) ID() string        { return e.id }
func (e *Episode) Name() string      { return e.name }
func (e *Episode) SortName() string  { return e.sortName }
func (e *Episode) Path() string      { return e.path }
func (e *Episode) BaseUrl() string   { return "" }
func (e *Episode) FirstVideo() int64 { return e.VideoTS }
func (e *Episode) LastVideo() int64  { return e.VideoTS }
func (e *Episode) Banner() string    { return "" }
func (e *Episode) Fanart() string    { return "" }
func (e *Episode) Folder() string    { return "" }
func (e *Episode) Poster() string    { return e.Thumb }
func (e *Episode) Logo() string      { return "" }
func (e *Episode) FileName() string  { return e.fileName }
func (e *Episode) FileSize() int64   { return e.fileSize }
func (e *Episode) Duration() int {
	e.LoadNfo()
	if e.Nfo != nil &&
		e.Nfo.FileInfo != nil &&
		e.Nfo.FileInfo.StreamDetails != nil &&
		e.Nfo.FileInfo.StreamDetails.Video != nil {
		return e.Nfo.FileInfo.StreamDetails.Video.DurationInSeconds
	}
	log.Printf("GetDuration(): no duration for episode %s\n", e.id)
	return 0
}
func (e *Episode) GetGenres() []string       { return []string{} }
func (e *Episode) GetYear() int              { return 0 }
func (e *Episode) GetRating() float32        { return 0 }
func (e *Episode) GetOfficialRating() string { return "" }
func (e *Episode) GetNfo() *Nfo              { return e.Nfo }

// LoadNfo loads the NFO file for the episode if not loaded already
func (e *Episode) LoadNfo() {
	loadNFO(&e.Nfo, e.nfoPath)
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

// Subs represents a subtitle file with its language and path.

type Subs struct {
	Lang string
	Path string
}

type Subtitles []Subs

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
