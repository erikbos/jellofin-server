// Parse filename for season / epside info.
// Example: easy.s01e04.mp4 -> season 1, episode .
package main

import (
	"fmt"
	"regexp"
	"strconv"
)

// pattern: ___.s03e04.___
var pat1 = regexp.MustCompile(`^.*[ ._][sS]([0-9]+)[eE]([0-9]+)[ ._].*$`)

// pattern: ___.s03e04e05.___ or ___.s03e04-e05.___
var pat2 = regexp.MustCompile(`^.*[. _[sS]([0-9]+)[eE]([0-9]+)-?[eE]([0-9]+)[. _].*$`)

// pattern: ___.2015.03.08.___
var pat3 = regexp.MustCompile(`^.*[ .]([0-9]{4})[.-]([0-9]{2})[.-]([0-9]{2})[ .].*$`)

// pattern: ___.308.___  (or 3x08) where first number is season.
var pat4 = regexp.MustCompile(`^.*[ .]([0-9]{1,2})x?([0-9]{2})[ .].*$`)

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

func parseEpisodeName(name string, seasonHint int, ep *Episode) (ok bool) {

	ok = true

	s := pat1.FindStringSubmatch(name)
	if len(s) > 0 {
		ep.Name = fmt.Sprintf("%sx%s", s[1], s[2])
		ep.SeasonNo = parseInt(s[1])
		ep.EpisodeNo = parseInt(s[2])
		return
	}

	s = pat2.FindStringSubmatch(name)
	if len(s) > 0 {
		ep.Name = fmt.Sprintf("%sx%s-%s", s[1], s[2], s[3])
		ep.SeasonNo = parseInt(s[1])
		ep.EpisodeNo = parseInt(s[2])
		ep.Double = true
		return
	}

	s = pat3.FindStringSubmatch(name)
	if len(s) > 0 {
		ep.Name = s[1] + "." + s[2] + "." + s[3]
		ep.SeasonNo = seasonHint
		ep.EpisodeNo = parseInt(s[1] + s[2] + s[3])
		return
	}

	s = pat4.FindStringSubmatch(name)
	if len(s) > 0 {
		sn := parseInt(s[1])
		if seasonHint < 0 || seasonHint == sn {
			ep.Name = fmt.Sprintf("%02dx%s", sn, s[2])
			ep.SeasonNo = sn
			ep.EpisodeNo = parseInt(s[2])
		}
		return
	}

	ok = false
	return
}
