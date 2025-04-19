package main

import (
	"log"
	"net/http"
	"strconv"
	"time"
)

// statusWriter proxies http.ResponseWriter
// and stores the requests status and length.
type statusWriter struct {
	http.ResponseWriter
	status int
	length int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (length int, err error) {
	if w.status == 0 {
		w.status = 200
	}
	length, err = w.ResponseWriter.Write(b)
	w.length += length
	return
}

// HttpLog calls ServeHTTP with a custom responsewriter that
// stores the requests status and length so we can log it.
func HttpLog(handle http.Handler) http.HandlerFunc {
	if handle == nil {
		handle = http.DefaultServeMux
	}
	return func(w http.ResponseWriter, request *http.Request) {
		start := time.Now()
		writer := statusWriter{w, 0, 0}
		handle.ServeHTTP(&writer, request)
		end := time.Now()
		latency := end.Sub(start)

		if writer.status > 200 {
			log.Printf("\n")
		}

		log.Printf("%v %s \"%s %s %s\" %d %d %s %v",
			end.Format("2006/01/02 15:04:05"),
			request.RemoteAddr,
			request.Method,
			request.URL.String(),
			request.Proto,
			writer.status,
			writer.length,
			strconv.Quote(request.Header.Get("User-Agent")),
			latency.Milliseconds())

		if writer.status > 200 {
			log.Printf("\n")
		}
	}
}
