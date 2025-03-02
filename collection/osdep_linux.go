//go:build !freebsd
// +build !freebsd

package collection

import (
	"syscall"
	"time"
)

func (fi *FileInfo) setCreatetime() {
	if fi.sys == nil {
		fi.createtime = fi.modtime
		return
	}
	stat := sys.(*syscall.Stat_t)
	atime := time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
	ctime := time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
	mtime := time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec)
	fi.createtime = mtime
	if atime.Before(fi.createtime) {
		fi.createtime = atime
	}
	if ctime.Before(fi.createtime) {
		fi.createtime = ctime
	}
}
