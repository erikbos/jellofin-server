// Support for `Kodi' style filesystem layout.
package collection

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/erikbos/jellofin-server/collection/metadata"
	"github.com/erikbos/jellofin-server/database/model"
	"github.com/erikbos/jellofin-server/idhash"
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
	eps *Episodes
	idx int
}

// buildMovies builds the movies in a collection. pace is the time to wait
// between processing each movie directory, to avoid overloading the filesystem.
// If pace is 0, no waiting is done.
func (cr *CollectionRepo) buildMovies(coll *Collection, pace time.Duration) (items []Item) {
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
		m := cr.buildMovie(coll, name)
		if m != nil {
			items = append(items, m)
		}
		if pace > 0 {
			time.Sleep(pace)
		}
	}
	coll.Items = items
	return
}

// buildMovie builds a movie item from a movie directory. It scans the directory
// for video files and images, and returns an Item.
func (cr *CollectionRepo) buildMovie(coll *Collection, dir string) (movie *Movie) {
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
	var filesize int64
	var created time.Time
	for _, f := range fi {
		s := isVideo.FindStringSubmatch(f.Name())
		if len(s) > 0 {
			ts := f.Createtime()
			if !ts.IsZero() {
				video = s[0]
				base = s[1]
				filesize = f.Size()
				created = ts

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
	if year == 0 && !created.IsZero() {
		year = created.Year()
	}
	if year == 0 {
		year = time.Now().Year()
	}

	movie = &Movie{
		id:       idhash.IdHash(mname),
		name:     mname,
		sortName: makeSortName(mname),
		// BaseUrl:    coll.BaseUrl,
		path:     dir,
		fileName: video,
		fileSize: filesize,
		created:  created,
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

		if isImage.MatchString(name) {
			if ext == "tbn" && aux == "" {
				aux = "poster"
			}
			switch aux {
			case `banner`:
				movie.banner = name
			case `fanart`:
				movie.fanart = name
			case `folder`:
				movie.folder = name
			case `poster`:
				movie.poster = name
			}
			continue
		}

		if ext == "srt" {
			if aux == "" || aux == "und" {
				aux = "zz"
			}
			movie.SrtSubs = append(movie.SrtSubs, Subs{
				Lang: aux,
				Path: name,
			})
			continue
		}

		if ext == "vtt" {
			if aux == "" || aux == "und" {
				aux = "zz"
			}
			movie.VttSubs = append(movie.VttSubs, Subs{
				Lang: aux,
				Path: name,
			})
			continue
		}

		if ext == "nfo" {
			movie.Metadata = metadata.NewNfo(path.Join(coll.Directory, dir, name))
			movie.Metadata.SetYear(year)
			continue
		}
	}

	// Setup a filename-based metadata handler in case of no metadata yet.
	if movie.Metadata == nil {
		movie.Metadata = metadata.NewFilename(movie.name, year)
	}

	cr.copySrtVttSubs(movie.SrtSubs, &movie.VttSubs)

	dbItemMovie := &model.Item{
		ID:    movie.id,
		Name:  movie.name,
		Year:  movie.Year(),
		Genre: strings.Join(movie.Genres(), ","),
	}

	cr.repo.DbLoadItem(dbItemMovie)

	return
}

// buildMovies builds the movies in a collection. pace is the time to wait
// between processing each movie directory, to avoid overloading the filesystem.
// If pace is 0, no waiting is done.
func (cr *CollectionRepo) buildShows(coll *Collection, pace time.Duration) (items []Item) {
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
		m := cr.buildShow(coll, name)
		if m != nil {
			items = append(items, m)
		}
		if pace > 0 {
			time.Sleep(pace)
		}
	}
	coll.Items = items
	return
}

func (cr *CollectionRepo) getSeason(show *Show, seasonNo int) (s *Season) {
	// find
	var i int
	for i := 0; i < len(show.Seasons); i++ {
		if seasonNo == show.Seasons[i].seasonno {
			return &(show.Seasons[i])
		}
	}

	// insert new
	name := idhash.IdHash(fmt.Sprintf("%s-season-%d", show.name, seasonNo))
	sn := &Season{
		id:       idhash.IdHash(name),
		name:     name,
		path:     show.path,
		seasonno: seasonNo,
		// Default images in case we do not have season-specific ones.
		seasonAllBanner: show.seasonAllBanner,
		seasonAllPoster: show.seasonAllPoster,
	}
	for i = 0; i < len(show.Seasons); i++ {
		if seasonNo < show.Seasons[i].seasonno {
			break
		}
	}
	tmp := make(Seasons, 0, len(show.Seasons)+1)
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

// showScanDir scans a show directory for episodes and images. It updates the
// show item with the found episodes and images.
func (cr *CollectionRepo) showScanDir(showDir, baseDir, seasonDir string, seasonHint int, show *Show) {
	d := path.Join(baseDir, seasonDir)
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
				cr.showScanDir(showDir, d, fn, sn, show)
				continue
			}

			// nfo file.
			if fn == "tvshow.nfo" {
				show.Metadata = metadata.NewNfo(path.Join(d, fn))
				continue
			}

			// other images.
			s = isImage.FindStringSubmatch(fn)
			if len(s) > 0 {
				switch s[1] {
				case "season-all-banner":
					show.seasonAllBanner = fn
				case "season-all-poster":
					show.seasonAllPoster = fn
				case "season-specials-poster":
					// Assign specials poster to season 0.
					if season := cr.getSeason(show, 0); season != nil {
						season.poster = path.Join(seasonDir, fn)
					}
				case "banner":
					show.banner = fn
				case "clearlogo":
					show.logo = fn
				case "fanart":
					show.fanart = fn
				case "folder":
					show.folder = fn
				case "poster":
					show.poster = fn
				}
			}
		}

		// now things that can only be found in a subdir
		// because they need context.
		if seasonHint >= 0 {
			s := isImage.FindStringSubmatch(fn)
			c := false
			if len(s) > 0 {
				p := path.Join(seasonDir, fn)
				switch s[1] {
				case "banner":
					season := cr.getSeason(show, seasonHint)
					season.banner = p
					c = true
				case "poster":
					season := cr.getSeason(show, seasonHint)
					season.poster = p
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
			season := cr.getSeason(show, sn)
			p := path.Join(seasonDir, fn)
			switch s[2] {
			case "poster":
				season.poster = p
			case "banner":
				season.banner = p
			default:
				// probably a poster.
				season.poster = p
			}
			continue
		}

		// episodes can be in main dir or subdir.
		s = isVideo.FindStringSubmatch(fn)
		if len(s) > 0 {
			ep := Episode{
				id:       idhash.IdHash(s[0]),
				path:     showDir,
				fileName: path.Join(seasonDir, fn),
				fileSize: f.Size(),
				baseName: s[1],
				Metadata: metadata.NewFilename(s[1], 0),
				created:  f.Createtime(),
			}
			if parseEpisodeName(s[1], seasonHint, &ep) {
				season := cr.getSeason(show, ep.SeasonNo)
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
		p := path.Join(seasonDir, name)

		if isImageExt.MatchString(ext) {
			if ext == "tbn" && aux == "" {
				aux = "thumb"
			}
			switch aux {
			case "thumb":
				ep.thumb = p
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
			ep.Metadata = metadata.NewNfo(path.Join(baseDir, seasonDir, name))
			continue
		}
	}
}

// buildShow builds a show item from a show directory.
// It scans the directory for episodes and images, and returns an Item
func (cr *CollectionRepo) buildShow(coll *Collection, dir string) (show *Show) {
	name := path.Base(dir)
	item := &Show{
		id:       idhash.IdHash(name),
		name:     name,
		sortName: makeSortName(name),
		// BaseUrl: coll.BaseUrl,
		path: dir,
	}
	d := path.Join(coll.Directory, dir)
	cr.showScanDir(dir, d, "", -1, item)

	for i := range item.Seasons {
		s := &(item.Seasons[i])
		// remove episodes without video
		eps := make(Episodes, 0, len(s.Episodes))
		for i := range s.Episodes {
			if s.Episodes[i].fileName != "" {
				eps = append(eps, s.Episodes[i])
			}
		}
		// and sort episodes
		s.Episodes = eps
		sort.Sort(Episodes(s.Episodes))
	}

	// remove seasons without episodes
	ssn := make(Seasons, 0, len(item.Seasons))
	for i := range item.Seasons {
		if len(item.Seasons[i].Episodes) > 0 {
			ssn = append(ssn, item.Seasons[i])
		}
	}
	// and sort seasons
	item.Seasons = ssn
	sort.Sort(Seasons(item.Seasons))

	if len(item.Seasons) > 0 {
		fs := item.Seasons[0]
		ls := item.Seasons[len(item.Seasons)-1]
		item.firstVideo = fs.Episodes[0].created
		item.lastVideo = ls.Episodes[len(ls.Episodes)-1].created
	}

	// If we have an NFO and at least one image, accept it.
	if item.Metadata != nil &&
		(item.fanart != "" || item.poster != "") {
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
	if !item.firstVideo.IsZero() {
		year = item.firstVideo.Year()
	}
	if year == 0 {
		year = time.Now().Year()
	}

	// Setup a filename-based metadata handler in case of no metadata yet.
	if item.Metadata == nil {
		item.Metadata = metadata.NewFilename(item.name, year)
	}
	item.Metadata.SetYear(year)

	dbItemShow := &model.Item{
		ID:    item.id,
		Name:  item.name,
		Year:  item.Metadata.Year(),
		Genre: strings.Join(item.Metadata.Genres(), ","),
	}
	cr.repo.DbLoadItem(dbItemShow)
	return
}

func (cr *CollectionRepo) copySrtVttSubs(srt Subtitles, vtt *Subtitles) {
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

func parseInt(s string) (i int) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		i = int(n)
	}
	return
}
