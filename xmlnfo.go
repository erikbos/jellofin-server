// Read `Kodi' style .NFO files

package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type Nfo struct {
	Title        string   `xml:"title,omitempty" json:"title,omitempty"`
	Id           string   `xml:"id,omitempty" json:"id,omitempty"`
	Runtime      string   `xml:"runtime,omitempty" json:"runtime,omitempty"`
	Mpaa         string   `xml:"mpaa,omitempty" json:"mpaa,omitempty"`
	YearString   string   `xml:"year,omitempty" json:"-"`
	Year         int      `xml:"-" json:"year,omitempty"`
	OTitle       string   `xml:"originaltitle,omitempty" json:"originaltitle,omitempty"`
	Plot         string   `xml:"plot,omitempty" json:"plot,omitempty"`
	Tagline      string   `xml:"tagline,omitempty" json:"tagline,omitempty"`
	Premiered    string   `xml:"premiered,omitempty" json:"premiered,omitempty"`
	Season       string   `xml:"season,omitempty" json:"season,omitempty"`
	Episode      string   `xml:"episode,omitempty" json:"episode,omitempty"`
	Aired        string   `xml:"aired,omitempty" json:"aired,omitempty"`
	Studio       string   `xml:"studio,omitempty" json:"studio,omitempty"`
	RatingString string   `xml:"rating,omitempty" json:"-"`
	Rating       float32  `xml:"-" json:"rating,omitempty"`
	VotesString  string   `xml:"votes,omitempty" json:"-"`
	Votes        int      `xml:"-" json:"votes,omitempty"`
	Genre        []string `xml:"genre,omitempty" json:"genre,omitempty"`
	Actor        []Actor  `xml:"actor,omitempty" json:"actor,omitempty"`
	Director     string   `xml:"director,omitempty" json:"director,omitempty"`
	Credits      string   `xml:"credits,omitempty" json:"credits,omitempty"`
	Thumb        string   `xml:"thumb,omitempty" json:"thumb,omitempty"`
	Fanart       []Thumb  `xml:"fanart,omitempty" json:"fanart,omitempty"`
	Banner       []Thumb  `xml:"banner,omitempty" json:"banner,omitempty"`
	Discart      []Thumb  `xml:"discart,omitempty" json:"discart,omitempty"`
	Logo         []Thumb  `xml:"logo,omitempty" json:"logo,omitempty"`
	// VidFileInfo	*VidFileInfo	`xml:"fileinfo,omitempty" json:"fileinfo,omitempty"`
}

type Thumb struct {
	Thumb string `xml:"thumb,omitempty" json:"thumb,omitempty"`
}

type Actor struct {
	Name string `xml:"name,omitempty" json:"name,omitempty"`
	Role string `xml:"role,omitempty" json:"role,omitempty"`
}

/*
type VidFileInfo struct {
	StreamDetails	*StreamDetails	`xml:"streamdetails,omitempty" json:"streamdetails,omitempty"`
}
type StreamDetails struct {
	Video		*VideoDetails	`xml:"video,omitempty" json:"video,omitempty"`
}
type VideoDetails struct {
	Codec		string		`xml:"codec,omitempty" json:"codec,omitempty"`
	Aspect		float32		`xml:"aspect,omitempty" json:"aspect,omitempty"`
	Width		int		`xml:"width,omitempty" json:"width,omitempty"`
	Height		int		`xml:"height,omitempty" json:"height,omitempty"`
}
*/

func decodeNfo(r io.ReadSeeker) (nfo *Nfo) {
	// this is a really dirty hack to partially support <xbmcmultiepisode>
	// for now. It just skips the tag and as a result parses just
	// the first episode in the multiepisode list.
	buf, err := io.ReadAll(r)
	if err != nil || len(buf) < 18 {
		return nil
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
		fmt.Println("Error unmarshalling from XML %v, %v", err, nfo)
		return
	}

	// Fix up genre.. bleh.
	needSplitup := false
	for _, g := range data.Genre {
		if strings.Index(g, ",") >= 0 ||
			strings.Index(g, "/") >= 0 {
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
	data.Rating = parseFloat32(data.RatingString)
	data.Votes = parseInt(data.VotesString)
	data.Year = parseInt(data.YearString)

	nfo = data
	return
}
