package collection

import (
	"strings"
	"unicode"

	"github.com/erikbos/jellofin-server/collection/metadata"
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
	// Duration returns the duration of the video in seconds.
	Duration() int
	// Genres returns the genres.

	metadata.VideoMetadata
	metadata.AudioMetadata

	Genres() []string
	Year() int
	Rating() float32
	OfficialRating() string
}

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
	// Metadata holds the metadata for the movie, e.g. from NFO file.
	Metadata metadata.Metadata

	SrtSubs Subtitles
	VttSubs Subtitles
}

func (m *Movie) ID() string        { return m.id }
func (m *Movie) Name() string      { return m.name }
func (m *Movie) SortName() string  { return m.sortName }
func (m *Movie) Path() string      { return m.path }
func (m *Movie) BaseUrl() string   { return m.baseUrl }
func (m *Movie) FirstVideo() int64 { return m.firstVideo }
func (m *Movie) Banner() string    { return m.banner }
func (m *Movie) Fanart() string    { return m.fanart }
func (m *Movie) Folder() string    { return m.folder }
func (m *Movie) Poster() string    { return m.poster }
func (m *Movie) Logo() string      { return m.logo }
func (m *Movie) FileName() string  { return m.fileName }
func (m *Movie) FilePath() string  { return m.path + "/" + m.fileName }
func (m *Movie) FileSize() int64   { return m.fileSize }
func (m *Movie) Duration() int     { return m.Metadata.Duration() }

// func (m *Movie) IsHD() bool              { return m.Metadata.IsHD() }
// func (m *Movie) Is4K() bool              { return m.Metadata.Is4K() }
func (m *Movie) VideoCodec() string      { return m.Metadata.VideoCodec() }
func (m *Movie) VideoBitrate() int       { return m.Metadata.VideoBitrate() }
func (m *Movie) VideoFrameRate() float64 { return m.Metadata.VideoFrameRate() }
func (m *Movie) VideoHeight() int        { return m.Metadata.VideoHeight() }
func (m *Movie) VideoWidth() int         { return m.Metadata.VideoWidth() }
func (m *Movie) AudioCodec() string      { return m.Metadata.AudioCodec() }
func (m *Movie) AudioBitrate() int       { return m.Metadata.AudioBitrate() }
func (m *Movie) AudioChannels() int      { return m.Metadata.AudioChannels() }
func (m *Movie) AudioLanguage() string   { return m.Metadata.AudioLanguage() }
func (m *Movie) Genres() []string        { return m.Metadata.Genres() }
func (m *Movie) Year() int               { return m.Metadata.Year() }
func (m *Movie) Rating() float32         { return m.Metadata.Rating() }
func (m *Movie) OfficialRating() string  { return m.Metadata.OfficialRating() }

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
	// Metadata holds the metadata for the show, e.g. from NFO file.
	Metadata metadata.Metadata

	SrtSubs Subtitles
	VttSubs Subtitles
	// Seasons contains the seasons in this TV show.
	Seasons Seasons
}

func (s *Show) ID() string              { return s.id }
func (s *Show) Name() string            { return s.name }
func (s *Show) SortName() string        { return s.sortName }
func (s *Show) Path() string            { return s.path }
func (s *Show) BaseUrl() string         { return s.baseUrl }
func (s *Show) FirstVideo() int64       { return s.firstVideo }
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
func (s *Show) VideoCodec() string      { return s.Metadata.VideoCodec() }
func (s *Show) VideoBitrate() int       { return s.Metadata.VideoBitrate() }
func (s *Show) VideoFrameRate() float64 { return s.Metadata.VideoFrameRate() }
func (s *Show) VideoHeight() int        { return s.Metadata.VideoHeight() }
func (s *Show) VideoWidth() int         { return s.Metadata.VideoWidth() }
func (s *Show) AudioCodec() string      { return s.Metadata.AudioCodec() }
func (s *Show) AudioBitrate() int       { return s.Metadata.AudioBitrate() }
func (s *Show) AudioChannels() int      { return s.Metadata.AudioChannels() }
func (s *Show) AudioLanguage() string   { return s.Metadata.AudioLanguage() }
func (s *Show) Genres() []string        { return s.Metadata.Genres() }
func (s *Show) Year() int               { return s.Metadata.Year() }
func (s *Show) Rating() float32         { return s.Metadata.Rating() }
func (s *Show) OfficialRating() string  { return s.Metadata.OfficialRating() }

// Season represents a season of a TV show, containing multiple episodes.
type Season struct {
	// id is the unique identifier of the season.
	id string
	// name is the human-readable name of the season.
	name string
	// path is the directory to the show(!), relative to collection root. (e.g. Casablanca)
	path string
	// seasonno is the season number, e.g., 1, 2, etc. 0 is used for specials.
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

func (season *Season) VideoCodec() string      { return "" }
func (season *Season) VideoBitrate() int       { return 0 }
func (season *Season) VideoFrameRate() float64 { return 0 }
func (season *Season) VideoHeight() int        { return 0 }
func (season *Season) VideoWidth() int         { return 0 }
func (season *Season) AudioCodec() string      { return "" }
func (season *Season) AudioBitrate() int       { return 0 }
func (season *Season) AudioChannels() int      { return 0 }
func (season *Season) AudioLanguage() string   { return "eng" }
func (season *Season) Genres() []string        { return []string{} }
func (season *Season) Year() int               { return 0 }
func (season *Season) Rating() float32         { return 0 }
func (season *Season) OfficialRating() string  { return "" }

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
	thumb string
	// Metadata holds the metadata for the episode, e.g. from NFO file.
	Metadata metadata.Metadata
	SrtSubs  Subtitles
	VttSubs  Subtitles
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
func (e *Episode) Poster() string    { return e.thumb }
func (e *Episode) Logo() string      { return "" }
func (e *Episode) FileName() string  { return e.fileName }
func (e *Episode) FileSize() int64   { return e.fileSize }
func (e *Episode) Number() int       { return e.EpisodeNo }
func (e *Episode) Duration() int     { return e.Metadata.Duration() }

// func (e *Episode) IsHD() bool              { return e.Metadata.IsHD() }
// func (e *Episode) Is4K() bool              { return e.Metadata.Is4K() }
func (e *Episode) VideoCodec() string      { return e.Metadata.VideoCodec() }
func (e *Episode) VideoBitrate() int       { return e.Metadata.VideoBitrate() }
func (e *Episode) VideoFrameRate() float64 { return e.Metadata.VideoFrameRate() }
func (e *Episode) VideoHeight() int        { return e.Metadata.VideoHeight() }
func (e *Episode) VideoWidth() int         { return e.Metadata.VideoWidth() }
func (e *Episode) AudioCodec() string      { return e.Metadata.AudioCodec() }
func (e *Episode) AudioBitrate() int       { return e.Metadata.AudioBitrate() }
func (e *Episode) AudioChannels() int      { return e.Metadata.AudioChannels() }
func (e *Episode) AudioLanguage() string   { return e.Metadata.AudioLanguage() }
func (e *Episode) Genres() []string        { return e.Metadata.Genres() }
func (e *Episode) Year() int               { return e.Metadata.Year() }
func (e *Episode) Rating() float32         { return e.Metadata.Rating() }
func (e *Episode) OfficialRating() string  { return e.Metadata.OfficialRating() }

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
