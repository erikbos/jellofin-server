package imageresize

import (
	"errors"
	"io"
	"os"
	"time"
)

type blobFile struct {
	fi   blobFileInfo
	blob []byte
	size int64
	pos  int64
}

type blobFileInfo struct {
	osfi     os.FileInfo
	size     int64
	openTime time.Time
}

type ioStatter interface {
	Stat() (os.FileInfo, error)
}

func NewBlobStringReader(data string, file interface{}) (f *blobFile) {
	return NewBlobBytesReader([]byte(data), file)
}

func NewBlobBytesReader(data []byte, file interface{}) (f *blobFile) {
	f = &blobFile{}
	f.blob = data
	f.size = int64(len(data))

	if stat, ok := file.(ioStatter); ok {
		f.fi.osfi, _ = stat.Stat()
	}
	f.fi.size = f.size
	f.fi.openTime = time.Now()

	return
}

func (f *blobFile) Read(p []byte) (n int, err error) {
	n = int(f.size - f.pos)
	if n <= 0 {
		err = io.EOF
		return
	}
	if n > len(p) {
		n = len(p)
	}
	copy(p[:n], f.blob[f.pos:int(f.pos)+n])
	f.pos += int64(n)
	if f.pos >= f.size {
		err = io.EOF
	}
	return
}

func (f *blobFile) Seek(offset int64, whence int) (pos int64, err error) {
	switch whence {
	case 0:
		pos = offset
	case 1:
		pos = f.pos + offset
	case 2:
		pos = f.size + offset
	default:
		pos = f.pos
	}
	if pos < 0 || pos > f.size {
		err = errors.New("seek position out of range")
	} else {
		f.pos = pos
	}
	return
}

func (f *blobFile) Stat() (fi os.FileInfo, err error) {
	return &f.fi, nil
}

func (f *blobFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, errors.New("not a directory")
}

func (f *blobFile) Close() error {
	return nil
}

func (f *blobFileInfo) Name() string {
	if f.osfi != nil {
		return f.osfi.Name()
	}
	return "blob"
}

func (f *blobFileInfo) Size() int64 {
	return f.size
}

func (f *blobFileInfo) Mode() os.FileMode {
	if f.osfi != nil {
		return f.osfi.Mode()
	}
	return 0444
}

func (f *blobFileInfo) ModTime() time.Time {
	if f.osfi != nil {
		return f.osfi.ModTime()
	}
	return f.openTime
}

func (f *blobFileInfo) IsDir() bool {
	return false
}

func (f *blobFileInfo) Sys() interface{} {
	if f.osfi != nil {
		return f.osfi.Sys()
	}
	return nil
}
