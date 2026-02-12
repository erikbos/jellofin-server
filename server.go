package main

import (
	"context"
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/erikbos/jellofin-server/collection"
	"github.com/erikbos/jellofin-server/database"
	"github.com/erikbos/jellofin-server/database/sqlite"
	"github.com/erikbos/jellofin-server/imageresize"
	"github.com/erikbos/jellofin-server/jellyfin"
	"github.com/erikbos/jellofin-server/muxnormalizer"
	"github.com/erikbos/jellofin-server/notflix"
)

type configFile struct {
	Listen struct {
		Address string
		Port    string
		TlsCert string
		TlsKey  string
	}
	Appdir   string
	Cachedir string
	Dbdir    string
	Database struct {
		Sqlite sqlite.ConfigFile `yaml:"sqlite"`
	} `yaml:"database"`
	Logfile     string
	Collections []struct {
		ID        string
		Name      string
		Type      string
		Directory string
		BaseUrl   string
		HlsServer string
	}
	Jellyfin struct {
		ServerID           string
		ServerName         string
		AutoRegister       bool
		ImageQualityPoster int
	}
}

func main() {
	const configFileNameKey = "config"

	// Set up viper for config file and command line flags
	viper.SetConfigType("yaml")
	viper.SetDefault("listen.port", "8096")
	viper.SetDefault("logfile", "/dev/stdout")

	pflag.String("config", "jellofin-server.yaml", "Path to configuration file.")
	viper.BindPFlag(configFileNameKey, pflag.Lookup("config"))
	pflag.Parse()

	// Read config file
	cf := viper.GetString(configFileNameKey)
	log.Printf("Using config file %s", cf)
	viper.SetConfigFile(cf)
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}
	var config configFile
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalf("Unable to decode config into struct: %v", err)
	}

	// Set up logging
	logfile := viper.GetString("logfile")
	log.Printf("Setting logfile to %s", logfile)
	switch logfile {
	case "none":
		log.SetOutput(io.Discard)
	default:
		f, err := os.OpenFile(logfile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		defer f.Close()
		log.SetOutput(f)
	}

	log.Printf("dbinit")
	var err error
	var repo database.Repository
	// Legacy support for Dbdir
	if config.Dbdir != "" {
		repo, err = database.New("sqlite", &sqlite.ConfigFile{
			Filename: path.Join(config.Dbdir, "tink-items.db"),
		})
	} else {
		repo, err = database.New("sqlite", config.Database.Sqlite)
	}
	if err != nil {
		log.Fatalf("database.New: %s", err.Error())
	}
	repo.StartBackgroundJobs(context.Background())

	// Initialize collection and add them to the collection manager
	collection := collection.New(&collection.Options{
		Repo: repo,
	})
	for _, coll := range config.Collections {
		collection.AddCollection(
			coll.Name,
			coll.ID,
			coll.Type,
			coll.Directory,
			coll.BaseUrl,
			coll.HlsServer,
		)
	}

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
		Repo:         repo,
		Imageresizer: resizer,
		Appdir:       config.Appdir,
	})
	n.RegisterHandlers(r)

	j := jellyfin.New(&jellyfin.Options{
		Collections:        collection,
		Repo:               repo,
		Imageresizer:       resizer,
		ServerPort:         config.Listen.Port,
		ServerID:           config.Jellyfin.ServerID,
		ServerName:         config.Jellyfin.ServerName,
		AutoRegister:       config.Jellyfin.AutoRegister,
		ImageQualityPoster: config.Jellyfin.ImageQualityPoster,
	})
	j.RegisterHandlers(r)

	r.Path("/robots.txt").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("User-agent: *\nDisallow: /\n"))
	})

	r.PathPrefix("/").Handler(http.FileServer(http.Dir(config.Appdir)))

	collection.Init()
	go collection.Background(context.Background())

	addr := net.JoinHostPort(config.Listen.Address, config.Listen.Port)

	// Add muxnormalizer middleware to canonicalize request paths and query parameters
	canon, err := muxnormalizer.New(r)
	if err != nil {
		log.Fatal(err)
	}
	server := HttpLog(canon.Middleware(r))

	if config.Listen.TlsCert != "" && config.Listen.TlsKey != "" {
		kpr, err := NewKeypairReloader(config.Listen.TlsCert, config.Listen.TlsKey)
		if err != nil {
			log.Fatalf("error loading keypair: %v", err)
		}

		srv := &http.Server{
			Addr:    addr,
			Handler: server,
			TLSConfig: &tls.Config{
				// Streamyfin's websocket connection still uses TLS1.2..
				MinVersion:     tls.VersionTLS12,
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
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	result := &keypairReloader{
		certPath: certPath,
		keyPath:  keyPath,
		cert:     &cert,
	}

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
