//go:build !freebsd && !linux

package main

func (fi *FileInfo) setCreatetime() {
	fi.createtime = fi.modtime
}
