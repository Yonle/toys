package main

import (
	"log"
	"net"
	"slices"

	"golang.org/x/sys/unix"
)

var BUF_SIZE = 4096
var L_ADDR = "[::1]:1111"

var fds []unix.PollFd
var conns = map[int]net.Conn{}
var blacklistWrite = map[int]struct{}{}
var trashcan = map[int]struct{}{}

func main() {
	ln, err := net.Listen("tcp", L_ADDR)
	if err != nil {
		panic(err)
	}

	defer ln.Close()

	lnfile, err := GetListenerFile(ln)

	if err != nil {
		panic(err)
	}

	defer lnfile.Close()
	lnfd := int(lnfile.Fd())
	if err := unix.SetNonblock(lnfd, true); err != nil {
		// bruh. we cannot NOT block it??
		panic(err)
	}

	log.Println("Now polling for TCP server on", L_ADDR)

	startPolling(ln, lnfd)
}

func startPolling(ln net.Listener, lnfd int) {
	// put listener in
	fds = append(fds, unix.PollFd{
		Fd:     int32(lnfd),
		Events: unix.POLLIN,
	})

	for { // welcome to the jungle
		n, err := unix.Poll(fds, -1)
		if err != nil {
			if err == unix.EINTR {
				// so skibidi, so gyat
				// fuck it. we ball.
				continue
			}
			log.Fatal("poll() fail: ", err)
		}

		if n == 0 {
			continue
		}

		log.Printf("there's %d fd doing new thing", n)

		for _, fd := range fds { // let's check each fds
			ifd := int(fd.Fd)

			if fd.Revents&unix.POLLIN != 0 { // something is coming
				// from whom?
				switch ifd {
				case lnfd: // from listener? accept new guest
					conn, err := ln.Accept()
					if err != nil {
						continue // nyeh.
					}

					connFile, err := GetConnFile(conn)
					if err != nil {
						continue // nyeh.
					}

					connFd := int(connFile.Fd())

					if err := unix.SetNonblock(connFd, true); err != nil {
						// we cannot NOT block it???
						continue // nyeh.
					}

					// attach the conn to fds
					fds = append(fds, unix.PollFd{
						Fd:     int32(connFd),
						Events: unix.POLLIN,
					})

					conns[connFd] = conn
					log.Printf("  new guest! look! it's fd is %d!", connFd)
				default: // beyond listener?
					if _, ok := conns[ifd]; !ok { // if we don't know, just ignore
						continue
					}

					log.Printf("  new event on %d!", ifd)

					// make buffer...
					buf := make([]byte, BUF_SIZE)

					for {
						n, err := unix.Read(ifd, buf)

						if err == unix.EAGAIN {
							log.Printf("    done reading data from %d :)", ifd)
							break
						}

						// TODO: handle half-read close

						if err != nil || n == 0 {
							trashcan[ifd] = struct{}{}
							break
						}

						d := buf[:n]

						broadcast(ifd, d)
					}
				}
			} else {
				if ifd == lnfd {
					continue // ignore listener
				}
				log.Printf("  ah... nothing on %d...", ifd)
			}
		}

		cleanupFds()
	}
}

func broadcast(bfd int, d []byte) {
	for _, pfd := range fds {
		fd := int(pfd.Fd)
		if _, dont := blacklistWrite[fd]; dont || fd == bfd {
			continue
		}

		if _, err := unix.Write(fd, d); err != nil {
			switch err {
			case unix.EPIPE:
				unix.Shutdown(fd, unix.SHUT_WR)
				blacklistWrite[fd] = struct{}{} // shh. don't talk
				log.Printf("    %d closed read on their end", fd)
			default:
				trashcan[bfd] = struct{}{}
			}
		}
	}
}

func cleanupFds() {
	// we moonwalk, coz if you try to delete slices forward,
	// the billie jeans blame you for the kids
	for i := len(fds) - 1; i >= 0; i-- {
		f := fds[i]
		fd := int(f.Fd)

		if _, ok := trashcan[fd]; !ok {
			continue
		}

		unix.Close(fd)
		conns[fd].Close()

		delete(conns, fd)
		delete(blacklistWrite, fd)
		fds = slices.Delete(fds, i, i+1)

		delete(trashcan, fd)

		log.Printf("  bye %d~", fd)
	}
}
