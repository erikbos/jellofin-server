package notflix

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"github.com/erikbos/jellofin-server/collection"
	"github.com/erikbos/jellofin-server/database"
	"github.com/erikbos/jellofin-server/imageresize"
)

type Options struct {
	Collections  *collection.CollectionRepo
	Db           *database.DatabaseRepo
	Imageresizer *imageresize.Resizer
	Appdir       string
}

type Notflix struct {
	collections  *collection.CollectionRepo
	db           *database.DatabaseRepo
	imageresizer *imageresize.Resizer
	Appdir       string
}

func New(o *Options) *Notflix {
	return &Notflix{
		collections:  o.Collections,
		imageresizer: o.Imageresizer,
		Appdir:       o.Appdir,
	}
}

func (n *Notflix) RegisterHandlers(r *mux.Router) {
	notFound := http.NotFoundHandler()
	gzip := handlers.CompressHandler

	r.Handle("/api", notFound)
	s := r.PathPrefix("/api/").Subrouter()
	s.HandleFunc("/collections", n.collectionsHandler)
	s.HandleFunc("/collection/{coll}", n.collectionHandler)
	s.HandleFunc("/collection/{coll}/genres", n.genresHandler)
	s.Handle("/collection/{coll}/items", gzip(http.HandlerFunc(n.itemsHandler)))
	s.Handle("/collection/{coll}/item/{item}", gzip(http.HandlerFunc(n.itemHandler)))

	r.Handle("/data", notFound)
	s = r.PathPrefix("/data/").Subrouter()
	s.HandleFunc("/{source}/{path:.*}", n.dataHandler)

	r.Handle("/v", notFound)
	r.PathPrefix("/v/").HandlerFunc(n.indexHandler)
}

func preCheck(w http.ResponseWriter, r *http.Request, keys ...string) (done bool) {
	fmt.Printf("precheck running\n")
	vars := mux.Vars(r)
	for _, k := range keys {
		if _, ok := vars[k]; !ok {
			http.Error(w, "500 Internal Server Error",
				http.StatusInternalServerError)
			done = true
			return
		}
	}
	switch r.Method {
	case "OPTIONS":
		setheaders(w.Header())
		done = true
	case "GET", "HEAD":
		setheaders(w.Header())
	default: // refuse the rest
		http.Error(w, "403 Access denied", http.StatusForbidden)
		done = true
	}
	return
}

func setheaders(h http.Header) {
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
}

func serveJSON(obj any, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	j := json.NewEncoder(w)
	j.SetIndent("", "  ")
	j.Encode(obj)
}

func (n *Notflix) collectionsHandler(w http.ResponseWriter, r *http.Request) {
	if preCheck(w, r) {
		return
	}
	cc := []Collection{}
	for _, c := range n.collections.GetCollections() {
		cc = append(cc, copyCollection(c))
	}
	serveJSON(cc, w)
}

func (n *Notflix) collectionHandler(w http.ResponseWriter, r *http.Request) {
	if preCheck(w, r, "coll") {
		return
	}
	vars := mux.Vars(r)
	c := n.collections.GetCollection(vars["coll"])
	if c == nil {
		http.Error(w, "404 Not Found", http.StatusNotFound)
		return
	}
	serveJSON(copyCollection(*c), w)
}

func (n *Notflix) itemsHandler(w http.ResponseWriter, r *http.Request) {
	if preCheck(w, r, "coll") {
		return
	}
	vars := mux.Vars(r)
	c := n.collections.GetCollection(vars["coll"])
	if c == nil {
		http.Error(w, "404 Not Found", http.StatusNotFound)
		return
	}

	var lastVideo int64
	for i := range c.Items {
		if c.Items[i].LastVideo > lastVideo {
			lastVideo = c.Items[i].LastVideo
		}
	}
	if lastVideo > 0 && checkEtagObj(w, r, time.UnixMilli(lastVideo)) {
		return
	}
	if r.Method == "HEAD" {
		return
	}
	serveJSON(copyItems(c.Items), w)
}

func (n *Notflix) itemHandler(w http.ResponseWriter, r *http.Request) {
	if preCheck(w, r, "coll", "item") {
		return
	}
	vars := mux.Vars(r)
	i := n.collections.GetItem(vars["coll"], vars["item"])
	if i == nil {
		http.Error(w, "404 Not Found", http.StatusNotFound)
		return
	}

	if i.LastVideo > 0 && checkEtagObj(w, r, time.UnixMilli(i.LastVideo)) {
		return
	}
	if r.Method == "HEAD" {
		return
	}

	r.ParseForm()
	doNfo := true
	if _, ok := r.Form["nonfo"]; ok {
		doNfo = false
	}

	if doNfo {
		i.LoadNfo()
	}

	i2 := copyItem(*i)
	if i.Seasons != nil {
		for _, s := range i.Seasons {
			i2.Seasons = append(i2.Seasons, copySeason(s, doNfo))
		}
	}
	serveJSON(&i2, w)
}

func (n *Notflix) genresHandler(w http.ResponseWriter, r *http.Request) {
	if preCheck(w, r, "coll") {
		return
	}
	vars := mux.Vars(r)
	c := n.collections.GetCollection(vars["coll"])
	if c == nil {
		http.Error(w, "404 Not Found", http.StatusNotFound)
		return
	}

	gc := c.GenreCount()
	serveJSON(gc, w)
}

func (n *Notflix) dataHandler(w http.ResponseWriter, r *http.Request) {
	if n.hlsHandler(w, r) {
		return
	}

	if preCheck(w, r, "source", "path") {
		return
	}
	vars := mux.Vars(r)
	c := n.collections.GetCollection(vars["source"])
	if c == nil {
		return
	}
	// dataDir := n.collections.GetDataDir(vars["source"])
	// if dataDir == "" {
	// 	http.Error(w, "404 Not Found", http.StatusNotFound)
	// 	return
	// }
	fn := path.Clean(path.Join(c.GetDataDir(), "/", vars["path"]))

	var err error
	var file http.File

	ext := ""
	i := strings.LastIndex(fn, ".")
	if i >= 0 {
		ext = fn[i+1:]
	}
	if ext == "srt" || ext == "vtt" {
		file, err = OpenSub(w, r, fn)
	} else {
		file, err = n.imageresizer.OpenFile(w, r, fn, 0)
	}
	defer file.Close()
	if err != nil {
		http.Error(w, "404 Not Found", http.StatusNotFound)
		return
	}

	fi, _ := file.Stat()
	if !fi.Mode().IsRegular() {
		http.Error(w, "403 Access denied", http.StatusForbidden)
		return
	}

	w.Header().Set("cache-control", "max-age=86400, stale-while-revalidate=300")
	if checkEtag(w, r, file) {
		return
	}

	http.ServeContent(w, r, fn, fi.ModTime(), file)
}

func (n *Notflix) indexHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, path.Join(n.Appdir, "index.html"))
}

// copyCollection populates a collection apiresponse
func copyCollection(c collection.Collection) Collection {
	cc := Collection{
		ID:   c.ID,
		Name: c.Name_,
		Type: c.Type,
	}
	return cc
}

func copyItems(c []*collection.Item) []Item {
	items := make([]Item, len(c))
	for i := range c {
		items[i] = copyItem(*c[i])
	}
	return items
}

func copyItem(item collection.Item) Item {
	ci := Item{
		ID:         item.ID,
		Name:       item.Name,
		Path:       item.Path,
		BaseUrl:    item.BaseUrl,
		Type:       item.Type,
		FirstVideo: item.FirstVideo,
		LastVideo:  item.LastVideo,
		Fanart:     item.Fanart,
		Poster:     item.Poster,
		Rating:     item.Rating,
		Genre:      item.Genres,
		Year:       item.Year,
		Video:      item.Video,
	}
	if item.Nfo != nil {
		ci.Nfo = ItemNfo{
			ID:        item.Nfo.Id,
			Title:     item.Nfo.Title,
			Plot:      item.Nfo.Plot,
			Genre:     item.Nfo.Genre,
			Premiered: item.Nfo.Premiered,
			MPAA:      item.Nfo.Mpaa,
			Aired:     item.Nfo.Aired,
			Studio:    item.Nfo.Studio,
		}
	}
	return ci
}

func copySeason(season collection.Season, doNfo bool) Season {
	cs := Season{
		SeasonNo: season.SeasonNo,
		Banner:   season.Banner,
		Fanart:   season.Fanart,
		Poster:   season.Poster,
	}

	cs.Episodes = make([]Episode, len(season.Episodes))
	for i := range season.Episodes {
		cs.Episodes[i] = copyEpisode(season.Episodes[i], doNfo)
	}
	return cs
}

func copyEpisode(episode collection.Episode, doNfo bool) Episode {
	ce := Episode{
		Name:      episode.Name,
		SeasonNo:  episode.SeasonNo,
		EpisodeNo: episode.EpisodeNo,
		Double:    episode.Double,
		SortName:  episode.SortName,
		Video:     episode.Video,
		Thumb:     episode.Thumb,
		// SrtSubs:   c.SrtSubs,
		// VttSubs:   c.VttSubs,
	}
	if doNfo {
		// log.Printf("Loading NFO for %s\n", episode.NfoPath)
		episode.LoadNfo()
		// log.Printf("NFO: %+v\n", ce.Nfo)
		if episode.Nfo != nil {
			ce.Nfo = EpisodeNfo{
				Title:   episode.Nfo.Title,
				Plot:    episode.Nfo.Plot,
				Season:  episode.Nfo.Season,
				Episode: episode.Nfo.Episode,
				Aired:   episode.Nfo.Aired,
			}
		}
	}
	return ce
}
