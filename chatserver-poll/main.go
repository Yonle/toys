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

// TODO: refactor this mess

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
			ifd := int(fds[i].Fd)

			if fds[i].Revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 { // shit gone wrong
				log.Printf("got POLLERR/POLLHUP/POLLNVAL on fd %d. yeeting away", ifd)
				makeItVanish(i)
				i--
				continue // take 2
			}

			if fds[i].Revents&unix.POLLRDHUP != 0 { // a client legit said "i don't wanna hear u but i will speak anyway"
				// from whom?
				switch ifd {
				case lnfd: // FROM LISTENER?!?!?!???
					panic("MAMAAAAAK THE KING HAS FALLEN!!!")
				default:
					//	fds[i].Events &^= (unix.POLLIN | unix.POLLRDHUP) // unsubscribe
					makeItVanish(i)
					i--
				}

				// log.Printf("that kid with fd %d shut his ears off, but still wanna hear us yap. gotcha", ifd)

				continue
			}

			if fds[i].Revents&unix.POLLIN != 0 { // something is coming
				// from whom?
				switch ifd {
				case lnfd: // from listener? accept new guest
					handleNewGuest()
				default: // beyond listener?
					removeDisFd := handleGuest(ifd)
					if removeDisFd {
						makeItVanish(i)
						i--
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

func handleGuest(nfd int) (removeDisFd bool) {
	buf := make([]byte, BUF_SIZE)

	for {
		n, err := unix.Read(nfd, buf)
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

		broadcast(nfd, buf[:n])
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
			_, err := unix.Write(fd, d)

			switch {
			case err == unix.EAGAIN:
				break
			case err == unix.EPIPE:
				unix.Shutdown(fd, unix.SHUT_WR)
				blacklistWrite[fd] = struct{}{} // shh. don't talk
				log.Printf("    %d closed read on their end", fd)
			case err != nil:
				// problem: the reader's forEach index is likely affected
				makeItVanish(i)
				i--
			}

			break
		}
	}
}

func makeItVanish(indx int) {
	fd := int(fds[indx].Fd)

	fds[indx] = fds[len(fds)-1] // move the latest crab here
	fds = fds[:len(fds)-1]      // make the last thing vanish

	unix.Close(fd)
	delete(blacklistWrite, fd)

	log.Printf("  bye %d~", fd)
}
