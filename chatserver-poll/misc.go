package main

import (
	"errors"
	"net"
	"os"
)

func GetListenerFile(ln net.Listener) (f *os.File, err error) {
	t, ok := ln.(*net.TCPListener)
	if !ok {
		return nil, errors.New("not a TCP listener")
	}

	return t.File()
}

func GetConnFile(conn net.Conn) (f *os.File, err error) {
	t, ok := conn.(*net.TCPConn)
	if !ok {
		return nil, errors.New("not a TCP connection")
	}

	return t.File()
}
