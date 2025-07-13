package main

import (
	"encoding/binary"
	"golang.org/x/sys/unix"
)

func (s *Session) MakeTCPConn(inettype int, sa unix.Sockaddr) (fd int, err error) {
	fd, err = unix.Socket(
		inettype,
		unix.SOCK_STREAM,
		unix.IPPROTO_TCP,
	)

	if err != nil {
		return
	}

	// TODO: make it nonblocking and put it to Epoll instance

	err = unix.Connect(fd, sa)

	if err != nil {
		return
	}

	return
}

func (s *Session) ConnectTo(atyp byte, addr, port []byte) (fd int, err error) {
	portInt := int(binary.BigEndian.Uint16(port))
	var inettype int
	var sa any

	switch atyp {
	case Atyp_Inet4:
		inettype = unix.AF_INET
		sa = &unix.SockaddrInet4{Port: portInt}
		copy(sa.(*unix.SockaddrInet4).Addr[:], addr)
	case Atyp_Inet6:
		inettype = unix.AF_INET6
		sa = &unix.SockaddrInet6{Port: portInt}
		copy(sa.(*unix.SockaddrInet6).Addr[:], addr)
	case Atyp_Name:
		inettype = unix.AF_UNSPEC
	}

	fd, err = s.MakeTCPConn(inettype, sa.(unix.Sockaddr))

	return
}
