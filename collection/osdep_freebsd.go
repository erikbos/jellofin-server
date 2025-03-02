package collection

import (
	"syscall"
	"time"
)

func (fi *FileInfo) setCreatetime() {
	if fi.sys == nil {
		return
	}
	stat := fi.sys.(*syscall.Stat_t)
	nsec := syscall.TimespecToNsec(stat.Ctimespec)
	fi.createtime = time.Unix(0, nsec)
	if fi.modtime.Before(fi.createtime) {
		fi.createtime = fi.modtime
	}
}
