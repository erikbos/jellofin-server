// Detect subtitle file encoding
// Convert .srt subtitles to .vtt or JSON.

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

var utf8BOM = "\xef\xbb\xbf"
var invalidANSI = make([]bool, 256, 256)
var badANSIchars = []byte{127, 129, 140, 141, 142, 143, 144, 154, 157,
	158, 159, 165, 197, 198, 225, 240, 254}

type subEntry struct {
	Id    int      `json:"id"`
	Start int      `json:"start"`
	End   int      `json:"end"`
	Lines []string `json:"lines"`
}

func init() {
	var c byte
	for c = 0; c < 32; c++ {
		invalidANSI[c] = true
	}
	for _, c = range badANSIchars {
		invalidANSI[c] = true
	}
}

func scanTime(word string, num *int) bool {
	var h, m, s, ms int
	_, err := fmt.Sscanf(word, "%d:%d:%d,%d", &h, &m, &s, &ms)
	if err != nil {
		return false
	}
	*num = (h*3600+m*60+s)*1000 + ms
	return true
}

func vttTime(ms int) string {
	s := ms / 1000
	h := s / 3600
	m := (s / 60) % 60
	s = s % 60
	ms = ms % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}

func parseSrt(file io.Reader) (subs []subEntry, utf8 bool) {

	isUTF8 := false
	isANSI := true
	state := 0
	e := subEntry{}

	b := bufio.NewReader(file)
	for {
		line, err := b.ReadString('\n')
		if err != nil {
			// resiliency.
			if len(e.Lines) > 0 {
				subs = append(subs, e)
			}
			break
		}
		l := len(line) - 1
		if l > 0 && line[l-1] == '\r' {
			l--
		}

		line = line[:l]
		if len(line) > 2 && line[0:3] == utf8BOM {
			isANSI = false
			isUTF8 = true
			line = line[3:]
		}
		if isANSI {
			for i := 0; i < len(line); i++ {
				if invalidANSI[line[i]] {
					isANSI = false
					break
				}
			}
		}

	outOfSync:
		switch state {
		case 0:
			_, err := fmt.Sscanf(line, "%d", &e.Id)
			if err != nil {
				state = 3
				goto outOfSync
			}
			state = 1
		case 1:
			words := strings.Fields(line)
			if len(words) != 3 || words[1] != "-->" {
				state = 3
				goto outOfSync
			}
			ok := scanTime(words[0], &e.Start)
			if ok {
				ok = scanTime(words[2], &e.End)
			}
			if !ok {
				state = 3
				goto outOfSync
			}
			state = 2
		case 2:
			if line == "" {
				subs = append(subs, e)
				state = 0
				e = subEntry{}
				break
			}
			e.Lines = append(e.Lines, line)
		case 3:
			e = subEntry{}
			if line == "" {
				state = 0
			}
		}
	}
	if isUTF8 || !isANSI {
		utf8 = true
	}
	return
}

func OpenSub(rw http.ResponseWriter, rq *http.Request, name string) (file http.File, err error) {
	i := strings.LastIndex(name, ".")
	ext := ""
	if i >= 0 {
		ext = name[i+1:]
	}

	if ext == "vtt" {
		file, err = os.Open(name)
		if err == nil {
			return
		}
		err = nil
	}

	if ext != "vtt" && ext != "srt" {
		err = os.ErrNotExist
		return
	}

	fn := name[:i] + ".srt"
	srtFile, err := os.Open(fn)
	if err != nil {
		return
	}
	subs, isUTF8 := parseSrt(srtFile)
	charset := "charset=ISO-8859-1"
	if isUTF8 {
		charset = "charset=utf-8"
	}

	accept := rq.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		rw.Header().Set("Content-Type", "application/json; "+charset)
		jsonBytes, err2 := json.MarshalIndent(subs, "", "  ")
		if err2 != nil {
			jsonBytes = []byte{'[', ']', '\n'}
		}
		file = NewBlobBytesReader(jsonBytes, srtFile)
		srtFile.Close()
		return
	}

	if ext == "srt" && !strings.Contains(accept, "text/vtt") {
		rw.Header().Set("Content-Type", "text/plain; "+charset)
		srtFile.Seek(0, 0)
		file = srtFile
		return
	}

	rw.Header().Set("Content-Type", "text/vtt; "+charset)

	lines := []string{"WEBVTT", ""}
	for _, s := range subs {
		tm := vttTime(s.Start) + " --> " + vttTime(s.End)
		lines = append(lines, tm)
		lines = append(lines, s.Lines...)
		lines = append(lines, "")
	}
	blob := strings.Join(lines, "\n") + "\n"
	file = NewBlobStringReader(blob, srtFile)
	srtFile.Close()
	return
}
