// MetadataFilename is a metadata handler that provides metadata based on the filename.
package metadata

import (
	"regexp"
	"strings"
	"time"
)

type MetadataFilename struct {
	// filename is the full path to the NFO file, e.g. "/mnt/media/casablanca.nfo"
	filename string
	// Name of the item (movie/series).
	name string
	// year is an optional override for the release year, could be derived from the file name.
	year int
	// height is the video height in pixels.
	height int
	// width is the video width in pixels.
	width int
	// videoCodec is the video coded.
	videoCodec string
	// audioCodec is the audio coded.
	audioCodec string
	// audiochannels is the number of audio channels.
	audiochannels int
}

// NewFilename creates a new metadata handler that provides metadata based on the filename.
func NewFilename(filename string, year int) *MetadataFilename {
	handler := &MetadataFilename{
		filename: filename,
		year:     year,
	}
	handler.parseFilename()
	return handler
}

// parseFilename guesses metadata from the filename.
func (n *MetadataFilename) parseFilename() {
	// We should attempt to extract title from the filename, removing common tags.
	n.name = n.filename

	var reCodec = regexp.MustCompile(`(?i)[hx].?264`)
	if reCodec.MatchString(n.name) {
		n.videoCodec = "h264"
	}
	if strings.Contains(n.name, "720") {
		n.height = 720
	}
	if strings.Contains(n.name, "1080") {
		n.height = 1080
	}
	if strings.Contains(n.name, "2160") || strings.Contains(n.name, "4K") {
		n.height = 2160
	}

	if strings.Contains(n.name, "aac") {
		n.audioCodec = "aac"
	}
	if strings.Contains(n.name, "2.0") {
		n.audiochannels = 2
	}
	if strings.Contains(n.name, "5.1") {
		n.audiochannels = 6
	}
}

// Duration returns the duration of the video in seconds.
func (n *MetadataFilename) Duration() time.Duration {
	return 0
}

// Title returns the title.
func (n *MetadataFilename) Title() string {
	return n.name
}

// GetGenres returns the genres.
func (n *MetadataFilename) Genres() []string {
	return []string{}
}

// SetYear sets the release year.
func (n *MetadataFilename) SetYear(year int) {
	n.year = year
}

// Year returns the release year.
func (n *MetadataFilename) Year() int {
	return n.year
}

// Rating returns the rating (0.0 - 10.0).
func (n *MetadataFilename) Rating() float32 {
	return 0
}

// OfficialRating returns the official rating (e.g. "PG-13").
func (n *MetadataFilename) OfficialRating() string {
	return ""
}

// Plot returns the plot/summary/description.
func (n *MetadataFilename) Plot() string {
	return ""
}

// Premiered returns the premiere date.
func (n *MetadataFilename) Premiered() time.Time {
	if n.year == 0 {
		return time.Time{}
	}
	return time.Date(n.year, time.January, 1, 0, 0, 0, 0, time.UTC)
}

// Actors returns map with actors and their role (e.g. Anthony Hopkins as Hannibal Lector).
func (n *MetadataFilename) Actors() map[string]string {
	return map[string]string{}
}

// Directors returns the directors.
func (n *MetadataFilename) Directors() []string {
	return []string{}
}

// Writers returns the writers.
func (n *MetadataFilename) Writers() []string {
	return []string{}
}

// Studios returns the studios.
func (n *MetadataFilename) Studios() []string {
	return []string{}
}

// Tagline returns the tagline.
func (n *MetadataFilename) Tagline() string {
	return ""
}

func (n *MetadataFilename) ProviderIDs() map[string]string {
	ids := make(map[string]string)
	return ids
}

// VideoBitrateBitrate returns the video bitrate in kbps.
func (n *MetadataFilename) VideoBitrate() int {
	return 0
}

// VideoFrameRate returns the video frame rate. (eg. 23.976).
func (n *MetadataFilename) VideoFrameRate() float64 {
	return 23.976
}

// VideoCodec returns the video codec (e.g. "h264").
func (n *MetadataFilename) VideoCodec() string {
	return n.videoCodec
}

// VideoHeight returns the video height in pixels.
func (n *MetadataFilename) VideoHeight() int {
	return n.height
}

// Video width returns the video width in pixels.
func (n *MetadataFilename) VideoWidth() int {
	return n.width
}

// AudioCodec returns the audio codec (e.g. "aac").
func (n *MetadataFilename) AudioCodec() string {
	return "unknown"
}

// AudioBitrate returns the audio bitrate in kbps.
func (n *MetadataFilename) AudioBitrate() int {
	return 0
}

// AudioChannels returns the number of audio channels (e.g. 6).
func (n *MetadataFilename) AudioChannels() int {
	return n.audiochannels
}

// AudioLanguage returns the audio language (e.g. "eng").
func (n *MetadataFilename) AudioLanguage() string {
	return "eng"
}
