package metadata

import "time"

type Metadata interface {
	// Title returns the title.
	Title() string
	// Plot returns the plot/summary/description.
	Plot() string
	// Tagline returns the tagline.
	Tagline() string
	// Studios returns the studios.
	Studios() []string
	// GetGenres returns the genres.
	Genres() []string
	// Year returns the release year.
	Year() int
	// SetYear sets the release year.
	SetYear(year int)
	// Premiered returns the premiere date.
	Premiered() time.Time
	// GetRating returns the rating (0.0 - 10.0).
	Rating() float32
	// OfficialRating returns the official rating (e.g. "PG-13").
	OfficialRating() string
	// ProviderIDs returns a map of provider IDs (e.g. {"imdb": "tt1234567", "tmdb": "12345"}).
	ProviderIDs() map[string]string
	// Duration returns the item duration in seconds.
	Duration() int

	VideoMetadata
	AudioMetadata
}

type VideoMetadata interface {
	// VideoCodec returns the video codec (e.g. "h264").
	VideoCodec() string
	// VideoBitrate returns the video bitrate in bps.
	VideoBitrate() int
	// VideoFrameRate returns the video frame rate. (eg. 23.976).
	VideoFrameRate() float64
	// VideoHeight returns the video height in pixels.
	VideoHeight() int
	// VideoWidth returns the video width in pixels.
	VideoWidth() int
}

type AudioMetadata interface {
	// AudioCodec returns the audio codec (e.g. "aac").
	AudioCodec() string
	// AudioBitrate returns the audio bitrate in bps.
	AudioBitrate() int
	// AudioChannels returns the number of audio channels (e.g. 6).
	AudioChannels() int
	// AudioLanguage returns the audio language (e.g. "en").
	AudioLanguage() string
}
