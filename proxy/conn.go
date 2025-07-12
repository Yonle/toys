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

func connectTo(atyp byte, addr, port []byte) (fd int, err error) {
	var inettype int

	switch atyp {
	case Atyp_Inet4:
		inettype = unix.AF_INET
	case Atyp_Inet6:
		inettype = unix.AF_INET6
	case Atyp_Name:
		// todo: make it resolve
	}

	return
}
