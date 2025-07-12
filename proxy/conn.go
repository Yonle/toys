package main

import (
	"golang.org/x/sys/unix"
)

func MakeTCPConn(inettype int, sa unix.Sockaddr) (fd int, err error) {
	fd, err = unix.Socket(
		inettype,
		(unix.SOCK_STREAM | unix.SOCK_CLOEXEC | unix.SOCK_NONBLOCK),
		unix.IPPROTO_TCP,
	)

	if err != nil {
		return
	}

	err = unix.Connect(fd, sa)

	if err != nil {
		return
	}

	return
}
