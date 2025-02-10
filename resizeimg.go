package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"sync"
	"syscall"

	"gopkg.in/gographics/imagick.v2/imagick"
)

var resizeMutexMap = make(map[string]*sync.Mutex)
var resizeMutexMapLock sync.Mutex

var isImg = regexp.MustCompile(`\.(png|jpg|jpeg|tbn)$`)
var tmpExt = ".tmp"

func resizeimg_init() {
	imagick.Initialize()
	tmpExt = fmt.Sprintf(".%d", os.Getpid())
}
func resizeimg_deinit() {
	imagick.Terminate()
}

func param2float(params map[string][]string, param string) (r float64) {
	if val, ok := params[param]; ok && len(val) > 0 {
		x, _ := strconv.ParseUint(val[0], 10, 64)
		r = float64(x)
	}
	return
}

func cacheName(file http.File) (r string) {
	fi, err := file.Stat()
	if err != nil {
		return
	}
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return
	}
	return fmt.Sprintf("%08x.%016x", stat.Dev, stat.Ino)
}

// get info about the original file (width x height) from the cache.
func cacheReadInfo(file http.File) (w float64, h float64) {
	if config.Cachedir == "" {
		return
	}
	cn := cacheName(file)
	if cn == "" {
		return
	}
	fn := fmt.Sprintf("%s/%s", config.Cachedir, cn)
	fh, err := os.Open(fn)
	if err != nil {
		return
	}
	var uw, uh uint
	_, err = fmt.Fscanf(fh, "%dx%d\n", &uw, &uh)
	if err == nil {
		w = float64(uw)
		h = float64(uh)
	}
	fh.Close()
	return
}

// write info about the original file (width x height) to the cache.
func cacheWriteInfo(file http.File, w float64, h float64) {
	if config.Cachedir == "" {
		return
	}
	cn := cacheName(file)
	if cn == "" {
		return
	}
	fn := fmt.Sprintf("%s/%s", config.Cachedir, cn)
	tmp := fn + tmpExt
	fh, err := os.Create(tmp)
	if err != nil {
		return
	}
	defer fh.Close()
	_, err = fmt.Fprintf(fh, "%.fx%.f\n", w, h)
	if err == nil {
		err = os.Rename(tmp, fn)
	}
	if err != nil {
		os.Remove(tmp)
	}
}

// see if we have the resized file in the cache.
func cacheRead(file http.File, w uint, h uint, q uint) (rfile http.File) {
	if config.Cachedir == "" {
		return
	}
	cn := cacheName(file)
	if cn == "" {
		return
	}
	fn := fmt.Sprintf("%s/%s:%dx%dq=%d", config.Cachedir, cn, w, h, q)
	rfile, err := os.Open(fn)
	if err != nil {
		rfile = nil
	}
	return
}

// store resized file in the cache.
func cacheWrite(file http.File, blob []byte, w uint, h uint, q uint) (rfile http.File) {
	if config.Cachedir == "" {
		return
	}
	cn := cacheName(file)
	if cn == "" {
		return
	}
	fn := fmt.Sprintf("%s/%s:%dx%dq=%d", config.Cachedir, cn, w, h, q)
	tmp := fn + tmpExt
	fh, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0666)
	if err != nil {
		return
	}
	_, err = fh.Write(blob)
	if err != nil {
		fh.Close()
		os.Remove(tmp)
		return
	}
	err = os.Rename(tmp, fn)
	if err != nil {
		fh.Close()
		os.Remove(tmp)
	}
	rfile = fh
	rfile.Seek(0, 0)
	return
}

// If the file is present, an image, and needs to be resized,
// then we return a handle to the resized image.
func OpenFile(rw http.ResponseWriter, rq *http.Request, name string) (file http.File, err error) {

	file, err = os.Open(name)
	if err != nil {
		return
	}

	// only plain files.
	fi, _ := file.Stat()
	if fi.IsDir() {
		return
	}

	// is it a supported image type.
	s := isImg.FindStringSubmatch(name)
	if len(s) == 0 {
		return
	}
	ctype := s[1]
	if ctype == "tbn" || ctype == "jpeg" {
		ctype = "jpg"
	}
	rw.Header().Set("Content-Type", "image/"+ctype)

	// do we want to resize.
	if rq.Method != "GET" || rq.URL.RawQuery == "" {
		return
	}

	// parse 'w', 'h', 'q' query parameters.
	params, _ := url.ParseQuery(rq.URL.RawQuery)
	mw := param2float(params, "mw")
	mh := param2float(params, "mh")
	w := param2float(params, "w")
	h := param2float(params, "h")
	q := param2float(params, "q")
	if mw+mh+w+h+q == 0 {
		return
	}

	// check cache if we have both width and height.
	// use maxwidth or maxheight if width or height is not set.
	cw := w
	ch := h
	if cw == 0 || (mw > 0 && cw > mw) {
		cw = mw
	}
	if ch == 0 || (mh > 0 && ch > mh) {
		ch = mh
	}
	if cw != 0 && ch != 0 {
		cf := cacheRead(file, uint(cw), uint(ch), uint(q))
		if cf != nil {
			file.Close()
			file = cf
			return
		}
	}

	wand := imagick.NewMagickWand()

	ow, oh := cacheReadInfo(file)
	if ow == 0 || oh == 0 {
		err = wand.PingImageFile(file.(*os.File))
		ow = float64(wand.GetImageWidth())
		oh = float64(wand.GetImageHeight())
		file.Seek(0, 0)
		if err != nil || oh == 0 || ow == 0 {
			return
		}
		cacheWriteInfo(file, ow, oh)
	}

	// if we do not have both wanted width and height,
	// we need to calculate them.
	if w == 0 || h == 0 {

		// aspect ratio
		ar := ow / oh

		// calculate width if not set
		if w == 0 && h > 0 {
			w = h * ar
		}
		// calculate height if not set
		if h == 0 && w > 0 {
			h = w / ar
		}
		if w == 0 && h == 0 {
			w = ow
			h = oh
		}

		// calculate both max width and max height.
		if mw != 0 || mh != 0 {
			if mh == 0 || (mw > 0 && mh*ar > mw) {
				mh = mw / ar
			}
			if mw == 0 || (mh > 0 && mw/ar > mh) {
				mw = mh * ar
			}
		}

		// clip
		if (mh > 0 && h > mh) || (mw > 0 && w > mw) {
			h = mh
			w = mw
		}

	}

	// image could be the right size and quality already.
	need_resize := uint(ow) != uint(w) || uint(oh) != uint(h)
	if !need_resize && q == 0 {
		return
	}

	// now that we have all parameters, check cache once more.
	cf := cacheRead(file, uint(w), uint(h), uint(q))
	if cf != nil {
		file.Close()
		file = cf
		return
	}

	resizeMutexMapLock.Lock()
	m, ok := resizeMutexMap[name]
	if !ok {
		m = &sync.Mutex{}
		resizeMutexMap[name] = m
	}
	resizeMutexMapLock.Unlock()
	m.Lock()
	defer m.Unlock()

	// read entire image.
	err = wand.ReadImageFile(file.(*os.File))
	file.Seek(0, 0)
	if err != nil {
		return
	}

	// resize.
	if need_resize {
		// err = wand.ResizeImage(uint(w), uint(h),
		// imagick.FILTER_LANCZOS, 1)
		err = wand.ThumbnailImage(uint(w), uint(h))
		if err != nil {
			return
		}
	}

	// set quality
	if q != 0 {
		err = wand.SetImageCompressionQuality(uint(q))
		if err != nil {
			return
		}
		// in case wand.ThumbnailImage() hasn't done this yet.
		if !need_resize {
			err = wand.StripImage()
			if err != nil {
				return
			}
		}
	}

	// Create "File"
	format := wand.GetImageFormat()
	wand.SetImageFormat(format)
	f := NewBlobBytesReader(wand.GetImageBlob(), file)
	f.wand = wand

	// Write cache file.
	cachefh := cacheWrite(file, f.blob, uint(w), uint(h), uint(q))
	if cachefh != nil {
		f.Close()
		file.Close()
		file = cachefh
		return
	}

	// no cache file, return in-memory file.
	file.Close()
	file = f
	return
}
