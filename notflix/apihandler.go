package notflix

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"github.com/miquels/notflix-server/collection"
	"github.com/miquels/notflix-server/database"
	"github.com/miquels/notflix-server/imageresize"
	"github.com/miquels/notflix-server/nfo"
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

func serveJSON(obj interface{}, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	j := json.NewEncoder(w)
	j.SetIndent("", "  ")
	j.Encode(obj)
}

func (n *Notflix) collectionsHandler(w http.ResponseWriter, r *http.Request) {
	if preCheck(w, r) {
		return
	}
	cc := []collection.Collection{}
	for _, c := range n.collections.GetCollections() {
		c.Items = nil
		cc = append(cc, c)
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
	cc := *c
	cc.Items = []*collection.Item{}
	serveJSON(cc, w)
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

	// copy items
	items := make([]collection.Item, len(c.Items))
	for i := range c.Items {
		items[i] = *c.Items[i]
		items[i].Seasons = []collection.Season{}
		items[i].Nfo = nil
	}

	// hack to show empty items list here.
	var itemsObj interface{} = items
	if len(items) == 0 {
		itemsObj = []string{}
	}
	serveJSON(itemsObj, w)
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

	// decode base NFO into a copy of `item' because we don't want the
	// nfo details to hang around in memory.
	i2 := *i
	if doNfo && i2.NfoPath != "" {
		file, err := os.Open(i2.NfoPath)
		if err == nil {
			i2.Nfo = nfo.Decode(file)
			file.Close()
		}
	}

	// In case of a tvshow, do a deep copy and decode episode NFO
	copy(i2.Seasons, i.Seasons)
	for si := range i2.Seasons {
		copy(i2.Seasons[si].Episodes, i.Seasons[si].Episodes)
		for ei := range i2.Seasons[si].Episodes {
			ep := i2.Seasons[si].Episodes[ei]
			if doNfo {
				if ep.NfoPath != "" {
					file, err := os.Open(ep.NfoPath)
					if err == nil {
						ep2 := ep
						ep2.Nfo = nfo.Decode(file)
						file.Close()
						i2.Seasons[si].Episodes[ei] = ep2
					}
				}
			}
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

	gc := make(map[string]int)
	for i := range c.Items {
		for _, g := range c.Items[i].Genre {
			if g == "" {
				continue
			}
			if v, found := gc[g]; !found {
				gc[g] = 1
			} else {
				gc[g] = v + 1
			}
		}
	}

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
