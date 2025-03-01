// Support for `Kodi' style filesystem layout.
package collection

import (
	//	"fmt"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/miquels/notflix-server/database"
	"github.com/miquels/notflix-server/idhash"
)

var isVideo = regexp.MustCompile(`^(.*)\.(divx|mov|mp4|MP4|m4u|m4v)$`)
var isImage = regexp.MustCompile(`^(.+)\.(jpg|jpeg|png|tbn)$`)
var isImageExt = regexp.MustCompile(`^(jpg|jpeg|png|tbn)$`)
var isSeasonImg = regexp.MustCompile(`^season([0-9]+)-?([a-z]+|)\.(jpg|jpeg|png|tbn)$`)
var isShowSubdir = regexp.MustCompile(`^S([0-9]+)|Specials([0-9]*)$`)
var isExt1 = regexp.MustCompile(`^(.*)()\.(png|jpg|jpeg|tbn|nfo|srt)$`)
var isExt2 = regexp.MustCompile(`^(.*)[.-]([a-z]+)\.(png|jpg|jpeg|tbn|nfo|srt)$`)
var isYear = regexp.MustCompile(` \(([0-9]+)\)$`)

type epMapType struct {
	eps *[]Episode
	idx int
}

func escapePath(p string) string {
	u := url.URL{Path: p}
	return u.EscapedPath()
}

func BuildMovies(coll *Collection, pace int) (items []*Item) {

	f, err := OpenDir(coll.Directory)
	if err != nil {
		return
	}
	defer f.Close()
	fi, _ := f.Readdir(0)
	if len(fi) == 0 {
		return
	}
	for _, f := range fi {
		name := f.Name()
		if (len(name) > 0 && name[:1] == ".") ||
			(len(name) > 1 && name[:2] == "+ ") {
			continue
		}
		m := buildMovie(coll, name)
		if m != nil {
			items = append(items, m)
		}
		if pace > 0 {
			d := time.Duration(int64(pace)) * time.Second
			time.Sleep(d)
		}
	}
	coll.Items = items
	return
}

func buildMovie(coll *Collection, dir string) (movie *Item) {

	d := path.Join(coll.Directory, dir)
	f, err := OpenDir(d)
	if err != nil {
		return
	}
	defer f.Close()
	fi, _ := f.Readdir(0)
	if len(fi) == 0 {
		return
	}
	mname := path.Base(dir)

	var base, video string
	var created int64
	for _, f := range fi {
		s := isVideo.FindStringSubmatch(f.Name())
		if len(s) > 0 {
			ts := f.CreatetimeMS()
			if ts > 0 {
				created = ts
				video = s[0]
				base = s[1]
			}
		}
	}
	if video == "" {
		return
	}

	s := isYear.FindStringSubmatch(dir)
	year := 0
	if len(s) > 0 {
		year = parseInt(s[1])
	}
	if year == 0 && created > 0 {
		t := time.Unix(created/1000, 0)
		year = t.Year()
	}
	if year == 0 {
		year = time.Now().Year()
	}

	movie = &Item{
		Id:         idhash.IdHash(mname),
		Name:       mname,
		Year:       year,
		BaseUrl:    coll.BaseUrl,
		Path:       escapePath(dir),
		Video:      escapePath(video),
		FirstVideo: created,
		LastVideo:  created,
		Type:       `movie`,
	}

	for _, f := range fi {
		name := f.Name()

		var aux string
		var ext string
		s := isExt1.FindStringSubmatch(name)
		if len(s) > 0 {
			ext = s[3]
			if s[1] != base {
				aux = s[1]
			}
		}
		if len(s) == 0 || s[1] != base {
			s = isExt2.FindStringSubmatch(name)
			if len(s) > 0 && s[1] == base {
				aux = s[2]
				ext = s[3]
			}
		}
		if ext == "" {
			continue
		}
		p := escapePath(name)

		if isImage.MatchString(name) {
			if ext == "tbn" && aux == "" {
				aux = "poster"
			}
			switch aux {
			case `banner`:
				movie.Banner = p
			case `fanart`:
				movie.Fanart = p
			case `folder`:
				movie.Folder = p
			case `poster`:
				movie.Poster = p
			}
			continue
		}

		if ext == "srt" {
			if aux == "" || aux == "und" {
				aux = "zz"
			}
			movie.SrtSubs = append(movie.SrtSubs, Subs{
				Lang: aux,
				Path: p,
			})
			continue
		}

		if ext == "vtt" {
			if aux == "" || aux == "und" {
				aux = "zz"
			}
			movie.VttSubs = append(movie.VttSubs, Subs{
				Lang: aux,
				Path: p,
			})
			continue
		}

		if ext == "nfo" {
			movie.NfoPath = path.Join(coll.Directory, dir, name)
			continue
		}
	}

	copySrtVttSubs(movie.SrtSubs, &movie.VttSubs)

	dbItemMovie := &database.Item{
		Id:    movie.Id,
		Name:  movie.Name,
		Year:  movie.Year,
		Genre: strings.Join(movie.Genre, ","),
	}

	database.DbLoadItem(dbItemMovie)

	return
}

func BuildShows(coll *Collection, pace int) (items []*Item) {

	f, err := OpenDir(coll.Directory)
	if err != nil {
		return
	}
	defer f.Close()
	fi, _ := f.Readdir(0)
	if len(fi) == 0 {
		return
	}
	for _, f := range fi {
		name := f.Name()
		if (len(name) > 0 && name[:1] == ".") ||
			(len(name) > 1 && name[:2] == "+ ") {
			continue
		}
		m := buildShow(coll, name)
		if m != nil {
			items = append(items, m)
		}
		if pace > 0 {
			d := time.Duration(int64(pace)) * time.Second
			time.Sleep(d)
		}
	}
	coll.Items = items
	return
}

func getSeason(show *Item, seasonNo int) (s *Season) {
	// find
	var i int
	for i = 0; i < len(show.Seasons); i++ {
		if seasonNo == show.Seasons[i].SeasonNo {
			return &(show.Seasons[i])
		}
	}

	// insert new
	sn := &Season{
		Id:       idhash.IdHash(fmt.Sprintf("%s-season-%d", show.Name, seasonNo)),
		SeasonNo: seasonNo,
	}
	for i = 0; i < len(show.Seasons); i++ {
		if seasonNo < show.Seasons[i].SeasonNo {
			break
		}
	}
	tmp := make([]Season, 0, len(show.Seasons)+1)
	tmp = append(tmp, show.Seasons[:i]...)
	tmp = append(tmp, *sn)
	tmp = append(tmp, show.Seasons[i:]...)
	show.Seasons = tmp
	s = &(show.Seasons[i])
	return
}

func epMatch(epMap map[string]epMapType, s []string) (ep *Episode, aux, ext string) {
	if len(s) < 4 {
		return
	}
	epx, ok := epMap[s[1]]
	if !ok {
		return
	}
	ep = &(*epx.eps)[epx.idx]
	aux = s[2]
	ext = s[3]
	return
}

func showScanDir(baseDir string, dir string, seasonHint int, show *Item) {

	d := path.Join(baseDir, dir)
	f, err := OpenDir(d)
	if err != nil {
		return
	}
	defer f.Close()
	fi, _ := f.Readdir(0)
	if len(fi) == 0 {
		return
	}

	epMap := make(map[string]epMapType)

	for _, f := range fi {
		fn := f.Name()

		// first things that can only be found in the
		// shows basedir, not in subdirs.
		if seasonHint < 0 {

			// S* subdir.
			s := isShowSubdir.FindStringSubmatch(fn)
			if len(s) > 0 {
				sn := parseInt(s[1])
				showScanDir(d, fn, sn, show)
				continue
			}

			// nfo file.
			if fn == "tvshow.nfo" {
				show.NfoPath = path.Join(d, fn)
				continue
			}

			// other images.
			s = isImage.FindStringSubmatch(fn)
			if len(s) > 0 {
				p := escapePath(fn)
				switch s[1] {
				case "season-all-banner":
					show.SeasonAllBanner = p
				case "season-all-poster":
					show.SeasonAllPoster = p
				case "banner":
					show.Banner = p
				case "fanart":
					show.Fanart = p
				case "folder":
					show.Folder = p
				case "poster":
					show.Poster = p
				}
			}
		}

		// now things that can only be found in a subdir
		// because they need context.
		if seasonHint >= 0 {
			s := isImage.FindStringSubmatch(fn)
			c := false
			if len(s) > 0 {
				p := escapePath(path.Join(dir, fn))
				switch s[1] {
				case "banner":
					season := getSeason(show, seasonHint)
					season.Banner = p
					c = true
				case "poster":
					season := getSeason(show, seasonHint)
					season.Poster = p
					c = true
				}
			}
			if c {
				continue
			}
		}

		// season image can be in main dir or subdir.
		s := isSeasonImg.FindStringSubmatch(fn)
		if len(s) > 0 {
			sn := parseInt(s[1])
			season := getSeason(show, sn)
			p := escapePath(path.Join(dir, fn))
			switch s[2] {
			case "poster":
				season.Poster = p
			case "banner":
				season.Banner = p
			default:
				// probably a poster.
				season.Poster = p
			}
			continue
		}

		// episodes can be in main dir or subdir.
		s = isVideo.FindStringSubmatch(fn)
		if len(s) > 0 {
			ep := Episode{
				Id:       idhash.IdHash(s[0]),
				Video:    escapePath(path.Join(dir, fn)),
				BaseName: s[1],
			}
			ep.VideoTS = f.CreatetimeMS()
			if parseEpisodeName(s[1], seasonHint, &ep) {
				season := getSeason(show, ep.SeasonNo)
				season.Episodes =
					append(season.Episodes, ep)
				epIndex := len(season.Episodes) - 1
				epMap[s[1]] = epMapType{
					eps: &season.Episodes,
					idx: epIndex,
				}
			}
		}
	}

	// Now scan the directory again for episode-related files.
	for _, f := range fi {

		name := f.Name()
		s := isExt1.FindStringSubmatch(name)
		ep, aux, ext := epMatch(epMap, s)
		if ep == nil {
			s = isExt2.FindStringSubmatch(name)
			ep, aux, ext = epMatch(epMap, s)
		}
		if ep == nil {
			continue
		}
		p := escapePath(path.Join(dir, name))

		if isImageExt.MatchString(ext) {
			if ext == "tbn" && aux == "" {
				aux = "thumb"
			}
			switch aux {
			case "thumb":
				ep.Thumb = p
			}
			continue
		}

		if ext == "srt" {
			if aux == "" || aux == "und" {
				aux = "zz"
			}
			ep.SrtSubs = append(ep.SrtSubs, Subs{
				Lang: aux,
				Path: p,
			})
			continue
		}

		if ext == "vtt" {
			if aux == "" || aux == "und" {
				aux = "zz"
			}
			ep.VttSubs = append(ep.VttSubs, Subs{
				Lang: aux,
				Path: p,
			})
			continue
		}

		if ext == "nfo" {
			ep.NfoPath = path.Join(baseDir, dir, name)
			continue
		}
	}
}

func buildShow(coll *Collection, dir string) (show *Item) {

	item := &Item{
		Id:      idhash.IdHash(path.Base(dir)),
		Name:    path.Base(dir),
		BaseUrl: coll.BaseUrl,
		Path:    escapePath(dir),
		Type:    `show`,
	}
	d := path.Join(coll.Directory, dir)
	showScanDir(d, "", -1, item)

	for i := range item.Seasons {
		s := &(item.Seasons[i])
		// remove episodes without video
		eps := make([]Episode, 0, len(s.Episodes))
		for i := range s.Episodes {
			if s.Episodes[i].Video != "" {
				eps = append(eps, s.Episodes[i])
			}
		}
		// and sort episodes
		s.Episodes = eps
		sort.Sort(byEpisode(s.Episodes))
	}

	// remove seasons without episodes
	ssn := make([]Season, 0, len(item.Seasons))
	for i := range item.Seasons {
		if len(item.Seasons[i].Episodes) > 0 {
			ssn = append(ssn, item.Seasons[i])
		}
	}
	// and sort seasons
	item.Seasons = ssn
	sort.Sort(bySeason(item.Seasons))

	if len(item.Seasons) > 0 {
		fs := item.Seasons[0]
		ls := item.Seasons[len(item.Seasons)-1]
		item.FirstVideo = fs.Episodes[0].VideoTS
		item.LastVideo = ls.Episodes[len(ls.Episodes)-1].VideoTS
	}

	// If we have an NFO and at least one image, accept it.
	if item.NfoPath != "" &&
		(item.Fanart != "" || item.Poster != "" || item.Thumb != "") {
		show = item
	}

	// Or if there is at least one video, accept it as well.
	for _, s := range item.Seasons {
		if len(s.Episodes) > 0 {
			show = item
		}
	}

	if show == nil {
		return
	}

	// guess the year in case it's not in the NFO file.
	year := 0
	if item.FirstVideo > 0 {
		t := time.Unix(item.FirstVideo/1000, 0)
		year = t.Year()
	}
	if year == 0 {
		year = time.Now().Year()
	}
	item.Year = year

	dbItemShow := &database.Item{
		Id:    item.Id,
		Name:  item.Name,
		Year:  item.Year,
		Genre: strings.Join(item.Genre, ","),
	}
	database.DbLoadItem(dbItemShow)
	return
}

func copySrtVttSubs(srt []Subs, vtt *[]Subs) {
	for i := range srt {
		sub := Subs{Lang: srt[i].Lang}
		path := srt[i].Path
		idx := strings.LastIndex(path, ".")
		if idx >= 0 {
			sub.Path = path[:idx] + ".vtt"
			*vtt = append(*vtt, sub)
		}
	}
}
