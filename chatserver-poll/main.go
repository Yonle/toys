package main

import (
	"log"

	"golang.org/x/sys/unix"
)

var BUF_SIZE = 4096
var L_ADDR = "[::1]:1111"

var l *Listener // listener
var fds []unix.PollFd
var blacklistWrite = make(map[int]struct{}) // int -> fd

func main() {
	var err error

	l, err = makeListener()
	if err != nil {
		log.Fatalln("failed to make listener:", err)
	}

	err = l.ListenInet6(L_ADDR)
	if err != nil {
		log.Fatalln("failed to listen:", err)
	}

	blacklistWrite[l.Fd] = struct{}{} // NEVER LET broadcast() WRITE

	log.Printf("Now polling for TCP server on %s (fd: %d)", L_ADDR, l.Fd)

	startPolling(l.Fd)
}

func startPolling(lnfd int) {
	// put listener in
	fds = append(fds, unix.PollFd{
		Fd:     int32(lnfd),
		Events: unix.POLLIN,
	})

	for { // welcome to the jungle
		n, err := unix.Poll(fds, -1)
		if err == unix.EINTR {
			// so skibidi, so gyat
			// fuck it. we ball.
			continue
		}

		if err != nil {
			log.Fatal("poll() fail: ", err)
		}

		if n == 0 {
			continue
		}

		for i := 0; i < len(fds); i++ { // let's check each fds
			fd := int(fds[i].Fd)

			if fds[i].Revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 { // shit gone wrong
				log.Printf("got POLLERR/POLLHUP/POLLNVAL on fd %d. yeeting away", fd)
				makeItVanish(i)
				unix.Close(fd)
				i--
				continue // take 2
			}

			if fds[i].Revents&unix.POLLRDHUP != 0 { // something half closed
				// from whom?
				switch fd {
				case lnfd: // FROM LISTENER?!?!?!???
					panic("MAMAAAAAK THE KING HAS FALLEN!!!")
				default: // it's hard to deal with half-read close due to schrodinger situation
					makeItVanish(i)
					unix.Close(fd)
					i--
				}
				continue
			}

			if fds[i].Revents&unix.POLLIN != 0 { // something is coming
				// from whom?
				switch fd {
				case lnfd: // from listener? accept new guest
					handleNewGuest()
				default: // beyond listener?
					removeDisFd := handleGuest(fd)
					if removeDisFd {
						unix.Close(fd)
						continue
					}
				}
			}
		}
	}
}

func handleNewGuest() {
	// let's accept new guest!
	nfd, _, err := l.Accept()

	if err != nil {
		log.Println("  failed to accept new fd:", err)
		return
	}

	fds = append(fds, unix.PollFd{
		Events: (unix.POLLIN | unix.POLLRDHUP),
		Fd:     int32(nfd),
	})

	log.Printf("  look! new guest! it's fd %d!", nfd)
}

func handleGuest(fd int) (removeDisFd bool) {
	buf := make([]byte, BUF_SIZE)

	for {
		n, err := unix.Read(fd, buf)
		switch {
		case err == unix.EAGAIN:
			return // we're done reading
		case err != nil:
			removeDisFd = true
			return
		}

		if n == 0 {
			removeDisFd = true
			return
		}

		broadcast(fd, buf[:n])
	}
}

func broadcast(bfd int, d []byte) {
	for i := 0; i < len(fds); i++ {
		f := fds[i]
		fd := int(f.Fd)
		if _, dont := blacklistWrite[fd]; dont || fd == bfd {
			continue
		}

		_, err := unix.Write(fd, d)

		switch {
		case err == unix.EPIPE:
			blacklistWrite[fd] = struct{}{} // shh. don't talk
			log.Printf("    %d closed read on their end", fd)
		case err != nil:
			unix.Close(fd)
		}
	}
}

func makeItVanish(indx int) {
	fd := int(fds[indx].Fd)

	fds[indx] = fds[len(fds)-1] // move the latest crab here
	fds = fds[:len(fds)-1]      // make the last thing vanish

	delete(blacklistWrite, fd)

	log.Printf("  bye %d~", fd)
}
