package imageresize

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"sync"
	"syscall"

	"github.com/disintegration/imaging"
)

type Options struct {
	Cachedir string
}
type Resizer struct {
	cachedir           string
	tmpExt             string
	resizeMutexMap     map[string]*sync.Mutex
	resizeMutexMapLock sync.Mutex
}

func New(config Options) *Resizer {
	r := &Resizer{
		cachedir:       config.Cachedir,
		resizeMutexMap: make(map[string]*sync.Mutex),
		tmpExt:         fmt.Sprintf(".%d", os.Getpid()),
	}
	return r
}

var isImg = regexp.MustCompile(`\.(png|jpg|jpeg|tbn)$`)

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
func (r *Resizer) cacheReadInfo(file http.File) (w float64, h float64) {
	if r.cachedir == "" {
		return
	}
	cn := cacheName(file)
	if cn == "" {
		return
	}
	fn := fmt.Sprintf("%s/%s", r.cachedir, cn)
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
func (r *Resizer) cacheWriteInfo(file http.File, w float64, h float64) {
	if r.cachedir == "" {
		return
	}
	cn := cacheName(file)
	if cn == "" {
		return
	}
	fn := fmt.Sprintf("%s/%s", r.cachedir, cn)
	tmp := fn + r.tmpExt
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
func (r *Resizer) cacheRead(file http.File, w, h, q uint) (rfile http.File) {
	if r.cachedir == "" {
		return
	}
	cn := cacheName(file)
	if cn == "" {
		return
	}
	fn := fmt.Sprintf("%s/%s:%dx%dq=%d", r.cachedir, cn, w, h, q)
	rfile, err := os.Open(fn)
	if err != nil {
		rfile = nil
	}
	return
}

// store resized file in the cache.
func (r *Resizer) cacheWrite(file http.File, blob []byte, w, h, q uint) (rfile http.File) {
	if r.cachedir == "" {
		return
	}
	cn := cacheName(file)
	if cn == "" {
		return
	}
	fn := fmt.Sprintf("%s/%s:%dx%dq=%d", r.cachedir, cn, w, h, q)
	tmp := fn + r.tmpExt
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
func (r *Resizer) OpenFile(rw http.ResponseWriter, rq *http.Request, name string,
	imageQuality int) (file http.File, err error) {
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

	// Hack: in case we did not get imagequality as queryparameter we can take it
	if imageQuality > 0 {
		q = float64(imageQuality)
	}

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
		cf := r.cacheRead(file, uint(cw), uint(ch), uint(q))
		if cf != nil {
			file.Close()
			file = cf
			return
		}
	}

	ow, oh := r.cacheReadInfo(file)
	if ow == 0 || oh == 0 {
		img, _, err2 := image.Decode(file)
		if err2 != nil {
			return nil, err
		}
		ow = float64(img.Bounds().Dx())
		oh = float64(img.Bounds().Dy())
		file.Seek(0, 0)
		if ow == 0 || oh == 0 {
			return
		}
		r.cacheWriteInfo(file, ow, oh)
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
	cf := r.cacheRead(file, uint(w), uint(h), uint(q))
	if cf != nil {
		file.Close()
		file = cf
		return
	}

	r.resizeMutexMapLock.Lock()
	m, ok := r.resizeMutexMap[name]
	if !ok {
		m = &sync.Mutex{}
		r.resizeMutexMap[name] = m
	}
	r.resizeMutexMapLock.Unlock()
	m.Lock()
	defer m.Unlock()

	// read entire image.
	img, _, err := image.Decode(file)
	file.Seek(0, 0)
	if err != nil {
		return
	}

	// resize.
	if need_resize {
		img = imaging.Resize(img, int(w), int(h), imaging.Lanczos)
	}

	// set quality
	var imageblob []byte
	if ctype == "jpg" {
		var buf bytes.Buffer
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: int(q)})
		imageblob = buf.Bytes()
	} else if ctype == "png" {
		var buf bytes.Buffer
		err = png.Encode(&buf, img)
		imageblob = buf.Bytes()
	}
	if err != nil {
		return
	}

	// Create "File"
	f := NewBlobBytesReader(imageblob, file)

	// Write cache file.
	cachefh := r.cacheWrite(file, f.blob, uint(w), uint(h), uint(q))
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
