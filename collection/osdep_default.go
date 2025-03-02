//go:build !freebsd && !linux

package collection

func (fi *FileInfo) setCreatetime() {
	fi.createtime = fi.modtime
}
