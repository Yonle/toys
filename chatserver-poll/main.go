package main

import (
	"log"
	"net"

	"golang.org/x/sys/unix"
)

var BUF_SIZE = 4096
var L_ADDR = "[::1]:1111"

var lnfd int
var fds []unix.PollFd
var blacklistWrite = make(map[int]struct{}) // int -> fd

// TODO: refactor this mess

func main() {
	var err error
	lnfd, err = unix.Socket(
		unix.AF_INET6,
		(unix.SOCK_STREAM | unix.SOCK_CLOEXEC | unix.SOCK_NONBLOCK),
		unix.IPPROTO_TCP)
	if err != nil {
		panic(err)
	}

	unix.SetsockoptInt(lnfd, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
	unix.SetsockoptInt(lnfd, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)

	var addr *net.TCPAddr
	addr, err = net.ResolveTCPAddr("tcp6", L_ADDR)
	if err != nil {
		panic(err)
	}

	var iface *net.Interface
	var zoneid uint32
	iface, err = net.InterfaceByName(addr.Zone)
	if err == nil {
		zoneid = uint32(iface.Index)
	}

	sockAddr := &unix.SockaddrInet6{
		Port:   addr.Port,
		Addr:   [16]byte(addr.IP.To16()),
		ZoneId: zoneid,
	}

	err = unix.Bind(lnfd, sockAddr)
	if err != nil {
		log.Fatalf("bind() failed: %v", err)
	}

	err = unix.Listen(lnfd, unix.SOMAXCONN)

	if err != nil {
		log.Fatalf("listen() failed: %v", err)
	}

	blacklistWrite[lnfd] = struct{}{} // NEVER LET broadcast() WRITE

	log.Printf("Now polling for TCP server on %s (fd: %d)", L_ADDR, lnfd)

	startPolling(lnfd)
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
	nfd, _, err := unix.Accept(lnfd)
	if err != nil {
		log.Println("warn: i couldn't accept connection:", err)
		return
	}

	if err := unix.SetNonblock(nfd, true); err != nil {
		unix.Close(nfd)
		log.Println("C_err: this fd couldn't set as noniblock:", err)
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
				continue
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
