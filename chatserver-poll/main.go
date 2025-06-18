package main

import (
	"log"
	"net"

	"golang.org/x/sys/unix"
)

var BUF_SIZE = 4096
var L_ADDR = "[::1]:1111"

var fds []unix.PollFd
var conns = map[int]net.Conn{}
var blacklistWrite = map[int]struct{}{} // int -> fd
var blacklistRead = map[int]struct{}{}  // int -> fd

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

	blacklistWrite[lnfd] = struct{}{} // NEVER LET broadcast() WRITE

	log.Printf("Now polling for TCP server on %s (fd: %d)", L_ADDR, lnfd)

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

		for i := 0; i < len(fds); i++ { // let's check each fds
			fd := fds[i]
			ifd := int(fd.Fd)

			if fd.Revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 { // shit gone wrong
				log.Printf("got POLLERR/POLLHUP/POLLNVAL on fd %d. yeeting away", ifd)
				makeItVanish(i)
				i--
				continue // take 2
			}

			if fd.Revents&unix.POLLRDHUP != 0 { // a client legit said "i don't wanna hear u but i will speak anyway"
				// from whom?
				switch ifd {
				case lnfd: // FROM LISTENER?!?!?!???
					panic("MAMAAAAAK THE KING HAS FALLEN!!!")
				default:
					blacklistRead[ifd] = struct{}{}                  // "son, don't talk to him"
					fds[i].Events &^= (unix.POLLIN | unix.POLLRDHUP) // unsubscribe
				}

				log.Printf("that kid with fd %d shut our mouth off, but still wanna yap. ok. gotcha.", ifd)

				continue
			}

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
						conn.Close()
						continue // nyeh.
					}

					connFd := int(connFile.Fd())

					if err := unix.SetNonblock(connFd, true); err != nil {
						// we cannot NOT block it???
						conn.Close()
						continue // nyeh.
					}

					// attach the conn to fds
					fds = append(fds, unix.PollFd{
						Fd:     int32(connFd),
						Events: (unix.POLLIN | unix.POLLRDHUP),
					})

					conns[connFd] = conn
					log.Printf("  new guest! look! it's fd is %d!", connFd)
				default: // beyond listener?
					if _, ok := conns[ifd]; !ok { // if we don't know, just ignore
						continue
					}

					if _, dont := blacklistRead[ifd]; dont { // momma said, "don't talk to him" coz "he say so". oh.
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

						if err != nil || n == 0 {
							makeItVanish(i)
							i--
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
	}
}

func broadcast(bfd int, d []byte) {
	for i := 0; i < len(fds); i++ {
		f := fds[i]
		fd := int(f.Fd)
		if _, dont := blacklistWrite[fd]; dont || fd == bfd {
			continue
		}

		for {
			if _, err := unix.Write(fd, d); err != nil {
				switch err {
				case unix.EAGAIN:
					continue
				case unix.EPIPE:
					unix.Shutdown(fd, unix.SHUT_WR)
					blacklistWrite[fd] = struct{}{} // shh. don't talk
					log.Printf("    %d closed read on their end", fd)
				default:
					// problem: the reader's forEach index is likely affected
					makeItVanish(i)
					i--
					continue
				}
			}

			break
		}
	}
}

func cleanupFd(fd int) {
	conns[fd].Close() // this closes fd already.

	delete(conns, fd)
	delete(blacklistWrite, fd)
	delete(blacklistRead, fd)
}

func makeItVanish(indx int) {
	fd := int(fds[indx].Fd)

	fds[indx] = fds[len(fds)-1] // move the latest crab here
	fds = fds[:len(fds)-1]      // make the last thing vanish

	cleanupFd(fd)
	log.Printf("  bye %d~", fd)
}
