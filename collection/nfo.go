// Read `Kodi' style .NFO files
package collection

import (
	"encoding/xml"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
)

type MetadataNfo struct {
	// filename is the full path to the NFO file, e.g. "/mnt/media/casablanca.nfo"
	filename string
	Nfo      *Nfo
}

func NewNfoMetadata(filename string) *MetadataNfo {
	return &MetadataNfo{
		filename: filename,
	}
}

// Duration returns the duration of the video in seconds.
func (n *MetadataNfo) Duration() int {
	return n.Nfo.FileInfo.StreamDetails.Video.DurationInSeconds
}

// GetGenres returns the genres.
func (n *MetadataNfo) GetGenres() []string {
	return n.Nfo.Genre
}

func (n *MetadataNfo) GetVotes() int {
	return n.Nfo.Votes

}
func (n *MetadataNfo) GetYear() int {
	return n.Nfo.Year
}

func (n *MetadataNfo) GetRating() float32 {
	return n.Nfo.Rating
}

func (n *MetadataNfo) GetOfficialRating() string {
	return n.Nfo.Mpaa
}

func (n *MetadataNfo) GetNfo() *Nfo {
	return n.Nfo
}

func (n *MetadataNfo) LoadNfo() {
	// NFO already loaded and parsed?
	if n.Nfo != nil {
		return
	}
	if file, err := os.Open(n.filename); err == nil {
		defer file.Close()
		n.Nfo, err = NfoDecode(file)
		if err != nil {
			log.Printf("Error parsing NFO file %s: %v\n", n.filename, err)
		}
	}
}

type Nfo struct {
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
	Rating       float32      `xml:"-"`
	VotesString  string       `xml:"votes,omitempty"`
	Votes        int          `xml:"-"`
	Genre        []string     `xml:"genre,omitempty"`
	Actor        []Actor      `xml:"actor,omitempty"`
	Director     string       `xml:"director,omitempty"`
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

func NfoDecode(r io.ReadSeeker) (*Nfo, error) {
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

	data := &Nfo{}
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

	data.Genre = NormalizeGenres(data.Genre)

	// Some non-string fields can be fscked up and explode the
	// XML decoder, so decode them after the fact.
	data.Rating = parseFloat32(data.RatingString)
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

func parseFloat32(s string) (i float32) {
	n, err := strconv.ParseFloat(s, 64)
	if err == nil {
		i = float32(n)
	}
	return
}
