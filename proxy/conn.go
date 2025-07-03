package main

import (
	"golang.org/x/sys/unix"
)

func MakeTCPConn(inettype int) (fd int, err error) {
	unix.Socket(
		inettype,
		(unix.SOCK_STREAM | unix.SOCK_CLOEXEC | unix.SOCK_NONBLOCK),
		unix.IPPROTO_TCP,
	)

	return
}
