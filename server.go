package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"log/syslog"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/XS4ALL/curlyconf-go"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"github.com/miquels/notflix-server/collection"
	"github.com/miquels/notflix-server/database"
	"github.com/miquels/notflix-server/imageresize"
	"github.com/miquels/notflix-server/jellyfin"
)

var configFile = "notflix-server.cfg"

type cfgMain struct {
	Listen      string
	Tls         bool
	TlsCert     string
	TlsKey      string
	Appdir      string
	Cachedir    string
	Dbdir       string
	Logfile     string
	Collections []collection.Collection `cc:"collection"`
	Jellyfin    jellyfin.Configuration
}

var config = cfgMain{
	Listen:  "127.0.0.1:8060",
	Logfile: "stdout",
}

func dataHandler(w http.ResponseWriter, r *http.Request) {
	if hlsHandler(w, r) {
		return
	}

	if preCheck(w, r, "source", "path") {
		return
	}
	vars := mux.Vars(r)
	dataDir := collection.GetDataDir(vars["source"])
	if dataDir == "" {
		http.Error(w, "404 Not Found", http.StatusNotFound)
		return
	}

	fn := path.Clean(path.Join(dataDir, "/", vars["path"]))

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
		file, err = resizer.OpenFile(w, r, fn, 0)
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

func indexHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, path.Join(config.Appdir, "index.html"))
}

func backgroundTasks() {
	for {
		updateCollections(1)
	}
}

var resizer *imageresize.Resizer

func main() {
	log.Printf("Parsing config file")

	p, err := curlyconf.NewParser(configFile, curlyconf.ParserNL)
	if err == nil {
		err = p.Parse(&config)
	}
	if err != nil {
		fmt.Println(err.(*curlyconf.ParseError).LongError())
		return
	}

	collection.New(config.Collections)

	log.Printf("Parsing flags")
	logfile := flag.String("logfile", config.Logfile,
		"Path of logfile. Use 'syslog' for syslog, 'stdout' "+
			"for standard output, or 'none' to disable logging.")
	flag.Parse()

	log.Printf("dbinit")
	err = database.Init(path.Join(config.Dbdir, "tink-items.db"))
	if err != nil {
		log.Fatalf("dbInit: %s\n", err)
	}

	resizer = imageresize.New(imageresize.ResizerConfig{
		Cachedir: config.Cachedir,
	})
	database.PlayState.Init()

	log.Printf("setting logfile")

	switch *logfile {
	case "syslog":
		logw, err := syslog.New(syslog.LOG_NOTICE, "notflix")
		if err != nil {
			log.Fatalf("error opening syslog: %v", err)
		}
		log.SetOutput(logw)
	case "none":
		log.SetOutput(ioutil.Discard)
	case "stdout":
	default:
		f, err := os.OpenFile(*logfile,
			os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		defer f.Close()
		log.SetOutput(f)
	}
	log.SetFlags(0)

	log.Printf("building mux")

	r := mux.NewRouter()
	notFound := http.NotFoundHandler()
	gzip := handlers.CompressHandler

	r.Handle("/api", notFound)
	s := r.PathPrefix("/api/").Subrouter()
	s.HandleFunc("/collections", collectionsHandler)
	s.HandleFunc("/collection/{coll}", collectionHandler)
	s.HandleFunc("/collection/{coll}/genres", genresHandler)
	s.Handle("/collection/{coll}/items",
		gzip(http.HandlerFunc(itemsHandler)))
	s.Handle("/collection/{coll}/item/{item}",
		gzip(http.HandlerFunc(itemHandler)))

	r.Handle("/data", notFound)
	s = r.PathPrefix("/data/").Subrouter()
	s.HandleFunc("/{source}/{path:.*}", dataHandler)

	r.Handle("/v", notFound)
	r.PathPrefix("/v/").HandlerFunc(indexHandler)

	j := jellyfin.New(&config.Jellyfin, config.Collections, resizer)
	j.RegisterHandlers(r)

	r.PathPrefix("/").Handler(http.FileServer(http.Dir(config.Appdir)))

	server := HttpLog(r)
	addr := config.Listen

	// XXX FIXME
	// if config.cachedir != "" {
	// 	go cleanCache(*datadir, config.cachedir, time.Hour)
	// }

	log.Printf("Initializing collections..")
	initCollections()

	go backgroundTasks()

	if config.Tls {
		log.Printf("Serving HTTPS on %s", addr)
		log.Fatal(http.ListenAndServeTLS(addr, config.TlsCert,
			config.TlsKey, server))
	} else {
		log.Printf("Serving HTTP on %s", addr)
		log.Fatal(http.ListenAndServe(addr, server))
	}
}

func updateCollections(pace int) {
	id := 1
	for i := range config.Collections {
		c := &(config.Collections[i])
		c.SourceId = id
		c.BaseUrl = fmt.Sprintf("/data/%d", id)
		switch c.Type {
		case collection.CollectionMovies:
			collection.BuildMovies(c, pace)
		case collection.CollectionShows:
			collection.BuildShows(c, pace)
		}
		id++
	}
}

func initCollections() {
	updateCollections(0)
}
