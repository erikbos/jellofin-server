package main

import (
	"crypto/tls"
	"flag"
	"io"
	"log"
	"log/syslog"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/XS4ALL/curlyconf-go"
	"github.com/gorilla/mux"

	"github.com/erikbos/jellofin-server/collection"
	"github.com/erikbos/jellofin-server/database"
	"github.com/erikbos/jellofin-server/imageresize"
	"github.com/erikbos/jellofin-server/jellyfin"
	"github.com/erikbos/jellofin-server/notflix"
)

var configFile = "jellofin-server.cfg"

type cfgMain struct {
	Listen      cfgListen
	Appdir      string
	Cachedir    string
	Dbdir       string
	Logfile     string
	Collections []collection.Collection `cc:"collection"`
	Jellyfin    struct {
		// Unique ID of the server, used in API responses
		ServerID string
		// ServerName is name of server returned in info responses
		ServerName string
		// Indicates if we should auto-register Jellyfin users
		AutoRegister bool
		// JPEG quality for posters
		ImageQualityPoster int
	}
}

type cfgListen struct {
	Address string // Address to listen on, empty for all interfaces
	Port    string // Port to listen on
	TlsCert string // Path to TLS certificate file
	TlsKey  string // Path to TLS key file
}

func main() {
	log.Printf("Parsing config file")
	config := cfgMain{
		Listen: cfgListen{
			Port: "8096",
		},
	}
	p, err := curlyconf.NewParser(configFile, curlyconf.ParserNL)
	if err == nil {
		err = p.Parse(&config)
	}
	if err != nil {
		log.Print(err.(*curlyconf.ParseError).LongError())
		return
	}

	log.Printf("Parsing flags")
	logfile := flag.String("logfile", config.Logfile,
		"Path of logfile. Use 'syslog' for syslog, 'stdout' "+
			"for standard output, or 'none' to disable logging.")
	flag.Parse()

	log.Printf("Setting logfile")
	switch *logfile {
	case "syslog":
		logw, err := syslog.New(syslog.LOG_NOTICE, "jellofin")
		if err != nil {
			log.Fatalf("error opening syslog: %v", err)
		}
		log.SetOutput(logw)
	case "none":
		log.SetOutput(io.Discard)
	case "":
		fallthrough
	case "stdout":
		log.SetFlags(0)
	default:
		f, err := os.OpenFile(*logfile,
			os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		defer f.Close()
		log.Printf("Setting logfile to %s", *logfile)
		log.SetOutput(f)
	}

	log.Printf("dbinit")
	database, err := database.New(&database.Options{
		Filename: path.Join(config.Dbdir, "tink-items.db"),
	})
	if err != nil {
		log.Fatalf("database.New: %s", err.Error())
	}
	go database.AccessTokenRepo.BackgroundJobs()
	go database.UserDataRepo.BackgroundJobs()

	collection := collection.New(&collection.Options{
		Collections: config.Collections,
		Db:          database,
	})

	resizer := imageresize.New(imageresize.Options{
		Cachedir: config.Cachedir,
	})
	// XXX FIXME
	// if config.cachedir != "" {
	// 	go cleanCache(*datadir, config.cachedir, time.Hour)
	// }

	log.Printf("building mux")

	r := mux.NewRouter()

	n := notflix.New(&notflix.Options{
		Collections:  collection,
		Db:           database,
		Imageresizer: resizer,
		Appdir:       config.Appdir,
	})
	n.RegisterHandlers(r)

	j := jellyfin.New(&jellyfin.Options{
		Collections:        collection,
		Db:                 database,
		Imageresizer:       resizer,
		ServerPort:         config.Listen.Port,
		ServerID:           config.Jellyfin.ServerID,
		ServerName:         config.Jellyfin.ServerName,
		AutoRegister:       config.Jellyfin.AutoRegister,
		ImageQualityPoster: config.Jellyfin.ImageQualityPoster,
	})
	j.RegisterHandlers(r)

	r.PathPrefix("/").Handler(http.FileServer(http.Dir(config.Appdir)))

	log.Printf("Initializing collections..")
	collection.Init()
	go collection.Background()

	addr := net.JoinHostPort(config.Listen.Address, config.Listen.Port)

	server := stripEmbyPath(HttpLog(r))

	if config.Listen.TlsCert != "" && config.Listen.TlsKey != "" {
		kpr, err := NewKeypairReloader(config.Listen.TlsCert, config.Listen.TlsKey)
		if err != nil {
			log.Fatalf("error loading keypair: %v", err)
		}

		srv := &http.Server{
			Addr:    addr,
			Handler: server,
			TLSConfig: &tls.Config{
				MinVersion:     tls.VersionTLS13,
				GetCertificate: kpr.GetCertificateFunc(),
			},
		}
		log.Printf("Serving HTTPS on %s", addr)
		log.Fatal(srv.ListenAndServeTLS("", ""))
	} else {
		log.Printf("Serving HTTP on %s", addr)
		log.Fatal(http.ListenAndServe(addr, server))
	}
}

type keypairReloader struct {
	certMu   sync.RWMutex
	cert     *tls.Certificate
	certPath string
	keyPath  string
}

// NewKeypairReloader creates a new keypair reloader that will reload the TLS certificate
// and key from the specified paths every 15 seconds. If the certificate cannot be loaded,
// it will log an error and keep the old certificate in use.
func NewKeypairReloader(certPath, keyPath string) (*keypairReloader, error) {
	result := &keypairReloader{
		certPath: certPath,
		keyPath:  keyPath,
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	result.cert = &cert

	go func() {
		for {
			// log.Printf("Attemping reloading TLS certificate and key from %q and %q", certPath, keyPath)
			time.Sleep(15 * time.Second)
			if err := result.maybeReload(); err != nil {
				log.Printf("Keeping old TLS certificate because the new one could not be loaded: %v", err)
			}
		}
	}()
	return result, nil
}

func (kpr *keypairReloader) GetCertificateFunc() func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		kpr.certMu.RLock()
		defer kpr.certMu.RUnlock()
		return kpr.cert, nil
	}
}

func (kpr *keypairReloader) maybeReload() error {
	newCert, err := tls.LoadX509KeyPair(kpr.certPath, kpr.keyPath)
	if err != nil {
		return err
	}
	kpr.certMu.Lock()
	defer kpr.certMu.Unlock()
	kpr.cert = &newCert
	return nil
}

// stripEmbyPath is a middleware that strips the "/emby" prefix from the request URL path.
func stripEmbyPath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/emby/") {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/emby")
		}
		next.ServeHTTP(w, r)
	})
}
