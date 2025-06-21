package main

import (
	"golang.org/x/sys/unix"
	"net"
)

type Listener struct {
	Fd int
}

func makeListener() (l *Listener, err error) {
	l = &Listener{}
	l.Fd, err = unix.Socket(
		unix.AF_INET6,
		(unix.SOCK_STREAM | unix.SOCK_CLOEXEC | unix.SOCK_NONBLOCK),
		unix.IPPROTO_TCP,
	)
	if err != nil {
		return
	}

	unix.SetsockoptInt(l.Fd, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
	unix.SetsockoptInt(l.Fd, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
	return
}

func (l *Listener) ListenInet6(l_addr string) (err error) {
	var addr *net.TCPAddr

	addr, err = net.ResolveTCPAddr("tcp6", l_addr)
	if err != nil {
		return
	}

	var iface *net.Interface
	var zoneid uint32
	iface, err = net.InterfaceByName(addr.Zone)
	if err == nil {
		zoneid = uint32(iface.Index)
	}
	err = nil // we could ignore it

	sockAddr := &unix.SockaddrInet6{
		Port:   addr.Port,
		Addr:   [16]byte(addr.IP.To16()),
		ZoneId: zoneid,
	}

	err = l.listen(sockAddr)
	return
}

func (l *Listener) listen(sockAddr unix.Sockaddr) (err error) {
	err = unix.Bind(l.Fd, sockAddr)
	if err != nil {
		return
	}

	err = unix.Listen(l.Fd, unix.SOMAXCONN)

	return
}

func (l *Listener) Close() (err error) {
	err = unix.Close(l.Fd)
	return
}

func (l *Listener) Accept() (nfd int, sa unix.Sockaddr, err error) {
	nfd, sa, err = unix.Accept(l.Fd)
	if err != nil {
		return
	}

	err = unix.SetNonblock(nfd, true)
	if err != nil {
		unix.Close(nfd)
	}

	return
}
