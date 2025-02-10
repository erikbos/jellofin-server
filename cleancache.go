package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var isCacheFile = regexp.MustCompile(`^[0-9a-f]{8}\.[0-9a-f]{16}$`)

func scanData(dir string, delay time.Duration) map[string]bool {
	m := make(map[string]bool)
	filepath.Walk(dir, func(path string, fi os.FileInfo, e error) (err error) {
		isdir := fi.Mode().IsDir()
		if e != nil {
			if isdir {
				err = filepath.SkipDir
			}
			return
		}
		if isdir {
			time.Sleep(delay)
			return
		}
		if !isImg.MatchString(path) {
			return
		}
		stat, ok := fi.Sys().(*syscall.Stat_t)
		if !ok {
			return
		}
		name := fmt.Sprintf("%08x.%016x", stat.Dev, stat.Ino)
		m[name] = true
		return nil
	})
	return m
}

func scanCache(dir string, exists map[string]bool, delay time.Duration) {
	filepath.Walk(dir, func(path string, fi os.FileInfo, e error) (err error) {
		isdir := fi.Mode().IsDir()
		if e != nil {
			if isdir {
				err = filepath.SkipDir
			}
			return
		}
		if isdir {
			time.Sleep(delay)
			return
		}
		name := fi.Name()
		i := strings.Index(name, ":")
		if i > 0 {
			name = name[:i]
		}
		if _, ok := exists[name]; !ok {
			if isCacheFile.MatchString(name) {
				os.Remove(path)
			}
		}
		return
	})
}

func dirExists(dir string) (ok bool) {
	fh, err := os.Open(dir)
	if err != nil {
		return
	}
	st, err := fh.Stat()
	if err != nil {
		return
	}
	ok = st.Mode().IsDir()
	fh.Close()
	return
}

func cleanCache(dataDir string, cacheDir string, sleep time.Duration) {
	for {
		if dirExists(dataDir) && dirExists(cacheDir) {
			m := scanData("/home/miquels/data/", 5*time.Millisecond)
			scanCache("/tmp/mxdav-img-cache", m, 20*time.Millisecond)
		}
		time.Sleep(sleep)
	}
}
