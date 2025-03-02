package notflix

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"syscall"
	"time"
)

func checkEtag(rw http.ResponseWriter, rq *http.Request, file http.File) bool {
	// create ETag based on name, inode number, and timestamp.
	fi, err := file.Stat()
	if err != nil {
		return false
	}
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	s := fmt.Sprintf("%s.%d.%d", rq.RequestURI,
		stat.Ino, fi.ModTime().Unix())
	m := md5.Sum([]byte(s))
	etag := hex.EncodeToString(m[:])

	rw.Header().Set("ETag", "\""+etag+"\"")

	if match := rq.Header.Get("If-None-Match"); match != "" {
		if strings.Contains(match, etag) {
			lm := fi.ModTime().Format(http.TimeFormat)
			rw.Header().Set("Last-Modified", lm)
			rw.WriteHeader(http.StatusNotModified)
			return true
		}
	}
	return false
}

func checkEtagObj(rw http.ResponseWriter, rq *http.Request, ts time.Time) bool {
	// create ETag based on timestamp.
	etag := fmt.Sprintf("\"%x\"", ts.Unix())
	rw.Header().Set("ETag", etag)

	// set last-modified as well for good measure
	lm := ts.Format(http.TimeFormat)
	rw.Header().Set("Last-Modified", lm)

	if match := rq.Header.Get("If-None-Match"); match != "" {
		if strings.Contains(match, etag) {
			rw.WriteHeader(http.StatusNotModified)
			return true
		}
	}
	return false
}
