// MetadataNfo is a metadata handler that can read Kodi style .NFO files.
package metadata

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

type MetadataNfo struct {
	// filename is the full path to the NFO file, e.g. "/mnt/media/casablanca.nfo"
	filename string
	// year is an optional override for the release year. Could be derived from the file name.
	year int
	// nfo is the parsed NFO data.
	nfo *nfo
}

// NewNfo creates a new metadata handler for the given NFO filename.
func NewNfo(filename string) *MetadataNfo {
	return &MetadataNfo{
		filename: filename,
	}
}

// Duration returns the duration of the video.
func (n *MetadataNfo) Duration() time.Duration {
	n.loadNfo()
	if n.nfo.Runtime != 0 {
		return time.Duration(n.nfo.Runtime*60) * time.Second
	}
	return time.Duration(n.nfo.FileInfo.StreamDetails.Video.DurationInSeconds) * time.Second
}

// Title returns the title.
func (n *MetadataNfo) Title() string {
	n.loadNfo()
	return n.nfo.Title
}

// GetGenres returns the genres.
func (n *MetadataNfo) Genres() []string {
	n.loadNfo()
	if len(n.nfo.Genre) == 0 {
		return nil
	}

	return n.nfo.Genre
}

// SetYear sets the release year.
func (n *MetadataNfo) SetYear(year int) {
	n.loadNfo()
	n.year = year
}

// Year returns the release year.
func (n *MetadataNfo) Year() int {
	n.loadNfo()
	if n.year != 0 {
		return n.year
	}
	return n.Premiered().Year()
}

// Rating returns the rating (0.0 - 10.0).
func (n *MetadataNfo) Rating() float32 {
	n.loadNfo()
	return float32(math.Round(n.nfo.Rating*10) / 10)
}

// OfficialRating returns the official rating (e.g. "PG-13").
func (n *MetadataNfo) OfficialRating() string {
	n.loadNfo()
	return n.nfo.Mpaa
}

// Plot returns the plot/summary/description.
func (n *MetadataNfo) Plot() string {
	n.loadNfo()
	return n.nfo.Plot
}

// Premiered returns the premiere date.
func (n *MetadataNfo) Premiered() time.Time {
	n.loadNfo()
	if n.nfo.Aired != "" {
		if parsedTime, err := n.parseTime(n.nfo.Aired); err == nil {
			return parsedTime
		}
	}
	if parsedTime, err := n.parseTime(n.nfo.Premiered); err == nil {
		return parsedTime
	}
	return time.Time{}
}

// Directors returns the directors.
func (n *MetadataNfo) Directors() []string {
	n.loadNfo()
	if len(n.nfo.Directors) == 0 {
		return nil
	}
	return n.nfo.Directors
}

// Studios returns the studios.
func (n *MetadataNfo) Studios() []string {
	n.loadNfo()
	if len(n.nfo.Studio) == 0 {
		return nil
	}
	return []string{n.nfo.Studio}
}

// Tagline returns the tagline.
func (n *MetadataNfo) Tagline() string {
	n.loadNfo()
	return n.nfo.Tagline
}

func (n *MetadataNfo) ProviderIDs() map[string]string {
	n.loadNfo()
	ids := make(map[string]string)
	for _, id := range n.nfo.UniqueIDs {
		if id.Default == "true" || id.Default == "1" {
			ids["default"] = id.Value
		} else if id.Type != "" && id.Value != "" {
			ids[strings.ToLower(id.Type)] = id.Value
		}
	}
	return ids
}

// VideoBitrateBitrate returns the video bitrate in kbps.
func (n *MetadataNfo) VideoBitrate() int {
	n.loadNfo()
	return n.nfo.FileInfo.StreamDetails.Video.Bitrate
}

// VideoFrameRate returns the video frame rate. (eg. 23.976).
func (n *MetadataNfo) VideoFrameRate() float64 {
	n.loadNfo()
	return math.Round(float64(n.nfo.FileInfo.StreamDetails.Video.FrameRate)*100) / 100
}

// VideoCodec returns the video codec (e.g. "h264").
func (n *MetadataNfo) VideoCodec() string {
	n.loadNfo()
	return n.nfo.FileInfo.StreamDetails.Video.Codec
}

// VideoHeight returns the video height in pixels.
func (n *MetadataNfo) VideoHeight() int {
	n.loadNfo()
	return n.nfo.FileInfo.StreamDetails.Video.Height
}

// Video width returns the video width in pixels.
func (n *MetadataNfo) VideoWidth() int {
	n.loadNfo()
	return n.nfo.FileInfo.StreamDetails.Video.Width
}

// AudioCodec returns the audio codec (e.g. "aac").
func (n *MetadataNfo) AudioCodec() string {
	n.loadNfo()
	return n.nfo.FileInfo.StreamDetails.Audio.Codec
}

// AudioBitrate returns the audio bitrate in kbps.
func (n *MetadataNfo) AudioBitrate() int {
	n.loadNfo()
	return n.nfo.FileInfo.StreamDetails.Audio.Bitrate
}

// AudioChannels returns the number of audio channels (e.g. 6).
func (n *MetadataNfo) AudioChannels() int {
	n.loadNfo()
	return n.nfo.FileInfo.StreamDetails.Audio.Channels
}

// AudioLanguage returns the audio language (e.g. "eng").
func (n *MetadataNfo) AudioLanguage() string {
	n.loadNfo()
	// return first 3 characters of language code
	if len(n.nfo.FileInfo.StreamDetails.Audio.Language) >= 3 {
		return n.nfo.FileInfo.StreamDetails.Audio.Language[0:3]
	}
	return "eng"
}

// loadNfo loads and parses the NFO file if not already done.
func (n *MetadataNfo) loadNfo() {
	// NFO already loaded and parsed?
	if n.nfo != nil {
		return
	}
	if file, err := os.Open(n.filename); err == nil {
		defer file.Close()
		n.nfo, err = NfoDecode(file)
		if err != nil {
			log.Printf("Error parsing NFO file %s: %v\n", n.filename, err)
		}
		// We ignore errors here, as we can work with partial data.
	}

	// We create empty structs to avoid nil pointer dereferences later.
	if n.nfo == nil {
		n.nfo = &nfo{}
	}
	if n.nfo.FileInfo == nil {
		n.nfo.FileInfo = &VidFileInfo{}
	}
	if n.nfo.FileInfo.StreamDetails == nil {
		n.nfo.FileInfo.StreamDetails = &StreamDetails{}
	}
	if n.nfo.FileInfo.StreamDetails.Video == nil {
		n.nfo.FileInfo.StreamDetails.Video = &VideoDetails{
			Codec: "unknown",
		}
	}
	if n.nfo.FileInfo.StreamDetails.Audio == nil {
		n.nfo.FileInfo.StreamDetails.Audio = &AudioDetails{
			Codec: "unknown",
		}
	}
}

// nfo represents the structure of a Kodi style .NFO file.
type nfo struct {
	Title        string       `xml:"title,omitempty"`
	Id           string       `xml:"id,omitempty"`
	Runtime      int          `xml:"runtime,omitempty"`
	Mpaa         string       `xml:"mpaa,omitempty"`
	YearString   string       `xml:"year,omitempty"`
	Year         int          `xml:"-"`
	OTitle       string       `xml:"originaltitle,omitempty"`
	Plot         string       `xml:"plot,omitempty"`
	Tagline      string       `xml:"tagline,omitempty"`
	Premiered    string       `xml:"premiered,omitempty"`
	Season       string       `xml:"season,omitempty"`
	Episode      string       `xml:"episode,omitempty"`
	Aired        string       `xml:"aired,omitempty"`
	Studio       string       `xml:"studio,omitempty"`
	RatingString string       `xml:"rating,omitempty"`
	Rating       float64      `xml:"-"`
	VotesString  string       `xml:"votes,omitempty"`
	Votes        int          `xml:"-"`
	Genre        []string     `xml:"genre,omitempty"`
	Actor        []Actor      `xml:"actor,omitempty"`
	Directors    []string     `xml:"director,omitempty"`
	Credits      string       `xml:"credits,omitempty"`
	UniqueIDs    []UniqueID   `xml:"uniqueid,omitempty"`
	Thumb        string       `xml:"thumb,omitempty"`
	Fanart       []Thumb      `xml:"fanart,omitempty"`
	Banner       []Thumb      `xml:"banner,omitempty"`
	Discart      []Thumb      `xml:"discart,omitempty"`
	Logo         []Thumb      `xml:"logo,omitempty"`
	FileInfo     *VidFileInfo `xml:"fileinfo,omitempty"`
}

type UniqueID struct {
	Type    string `xml:"type,attr"`
	Default string `xml:"default,attr"`
	Value   string `xml:",chardata"`
}

type Thumb struct {
	Thumb string `xml:"thumb,omitempty"`
}

type Actor struct {
	Name  string `xml:"name,omitempty"`
	Role  string `xml:"role,omitempty"`
	Thumb string `xml:"thumb,omitempty"`
}

type VidFileInfo struct {
	StreamDetails *StreamDetails `xml:"streamdetails,omitempty"`
}
type StreamDetails struct {
	Video *VideoDetails `xml:"video,omitempty"`
	Audio *AudioDetails `xml:"audio,omitempty"`
}
type VideoDetails struct {
	Codec             string  `xml:"codec,omitempty"`
	Bitrate           int     `xml:"bitrate,omitempty"`
	Aspect            float32 `xml:"aspect,omitempty"`
	Width             int     `xml:"width,omitempty"`
	Height            int     `xml:"height,omitempty"`
	FrameRate         float32 `xml:"framerate,omitempty"`
	DurationInSeconds int     `xml:"durationinseconds,omitempty"`
}

type AudioDetails struct {
	Bitrate  int    `xml:"bitrate,omitempty"`
	Channels int    `xml:"channels,omitempty"`
	Codec    string `xml:"codec,omitempty"`
	Language string `xml:"language,omitempty"`
}

func NfoDecode(r io.ReadSeeker) (*nfo, error) {
	// this is a really dirty hack to partially support <xbmcmultiepisode>
	// for now. It just skips the tag and as a result parses just
	// the first episode in the multiepisode list.
	buf, err := io.ReadAll(r)
	if err != nil || len(buf) < 18 {
		return nil, err
	}
	if string(buf[:18]) == "<xbmcmultiepisode>" {
		buf = buf[18:]
	}

	// as we're going to encode to JSON, make sure it's valid UTF-8
	txt := strings.ToValidUTF8(string(buf), "�")

	data := &nfo{}
	d := xml.NewDecoder(strings.NewReader(txt))
	d.Strict = false
	d.AutoClose = xml.HTMLAutoClose
	d.Entity = xml.HTMLEntity

	err = d.Decode(data)
	// fmt.Printf("data: %+v\nxmlData: %s\n", data, string(xmlData))
	if err != nil {
		// fmt.Printf("Error unmarshalling from XML %v, %v\n", err, nfo)
		return nil, err
	}

	// Fix up genre.. bleh.
	needSplitup := false
	for _, g := range data.Genre {
		if strings.Contains(g, ",") || strings.Contains(g, "/") {
			needSplitup = true
			break
		}
	}
	if needSplitup {
		genre := make([]string, 0)
		for _, g := range data.Genre {
			s := strings.Split(g, "/")
			if len(s) == 1 {
				s = strings.Split(g, ",")
			}
			for _, g2 := range s {
				genre = append(genre, strings.TrimSpace(g2))
			}
		}
		data.Genre = genre
	}

	data.Genre = normalizeGenres(data.Genre)

	// Some non-string fields can be fscked up and explode the
	// XML decoder, so decode them after the fact.
	data.Rating = parseFloat64(data.RatingString)
	data.Votes = parseInt(data.VotesString)
	data.Year = parseInt(data.YearString)

	return data, nil
}

func parseInt(s string) (i int) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		i = int(n)
	}
	return
}

func parseFloat64(s string) (i float64) {
	i, _ = strconv.ParseFloat(s, 64)
	return
}

func (n *MetadataNfo) parseTime(input string) (time.Time, error) {
	timeFormats := []string{
		"15:04:05",
		"2006-01-02",
		"2006/01/02",
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		"02 Jan 2006",
		"02 Jan 2006 15:04:05",
	}

	// Try each format until one succeeds
	for _, format := range timeFormats {
		if parsedTime, err := time.Parse(format, input); err == nil {
			return parsedTime, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time %s in %s", input, n.filename)
}
