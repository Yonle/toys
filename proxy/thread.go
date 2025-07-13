package main

import (
	"log"
	"sync"

	"github.com/Yonle/go-epoll"
	"golang.org/x/sys/unix"
)

type Thread struct {
	L *Listener
	E *epoll.Instance

	sess *sync.Map // *Session{} (SOCKS5 connection)
}

func MakeThread() (t *Thread, err error) {
	t = &Thread{
		sess: &sync.Map{},
	}

	t.E, err = epoll.NewInstance(0)
	return
}

func (t *Thread) Listen(L_ADDR string) (err error) {
	t.L, err = makeListener()
	if err != nil {
		return
	}

	ev := epoll.MakeEvent(t.L.Fd, (unix.EPOLLIN | unix.EPOLLRDHUP))
	err = t.E.Add(t.L.Fd, ev)
	if err != nil {
		return
	}

	err = t.L.ListenInet6(L_ADDR)
	if err != nil {
		return
	}

	return
}

func (t *Thread) StartWaiting() (err error) {
	events := make([]unix.EpollEvent, 512)

	var n int
	for {
		n, err = t.E.Wait(events, -1)
		if err == unix.EINTR {
			continue
		}

		if err != nil {
			return
		}

		if n == 0 {
			continue
		}

		for i := 0; i < n; i++ {
			e := events[i]
			fd := int(e.Fd)

			if e.Events&(unix.EPOLLHUP|unix.EPOLLERR|unix.EPOLLRDHUP) != 0 {
				t.close(fd)
				continue
			}

			if e.Events&unix.EPOLLIN != 0 { // something is coming
				switch fd {
				case t.L.Fd: // from listener
					t.handle_EPOLLIN_LN()
				default: // from client
					s, _ := t.sess.Load(fd)
					t.handle_EPOLLIN_C(s.(*Session))
					t.handle_session_stuffs(s.(*Session))
				}
			}
		}
	}
}

func (t *Thread) handle_EPOLLIN_LN() {
	// let's accept new guest!
	nfd, _, err := t.L.Accept()

	if err != nil {
		log.Println("  failed to accept new fd:", err)
		return
	}

	ev := epoll.MakeEvent(nfd, (unix.EPOLLIN | unix.EPOLLRDHUP))
	t.E.Add(nfd, ev)
	log.Printf("  look! new guest! it's fd %d!", nfd)

	// we make session
	t.sess.Store(nfd, t.MakeSession(nfd))
}

func (t *Thread) handle_EPOLLIN_C(s *Session) {
	n, err := unix.Read(s.Fd, s.rb[s.rl:])

	s.rl += n

	switch err {
	case nil:
	case unix.EAGAIN:
		return
	default:
		s.State = StateDone
		return
	}

	if s.rl == 0 {
		s.State = StateDone
		return
	}

	log.Println("Buf:", s.rb[:s.rl])

	switch s.State {
	case StateInit:
		switch s.CheckAuth() {
		case Yeet_Die: // yeet
			s.State = StateDone
			log.Println("YEET THE CONNECTION")
			return
		case Yeet_TryAgain: // incomplete data (EAGAIN)
			log.Println("Try again :>")
			return
		case Yeet_InvalidThenDie:
			log.Println("Invalid. YEET")
			s.State = StateDone
			return
		}
	case StateNeedCmd:
		s.CheckCmd()
	}
}

func (t *Thread) handle_session_stuffs(s *Session) {
	if s.sl != 0 {
		log.Println(s.sl, s.sb)
		unix.Write(s.Fd, s.sb[:s.sl]) // TODO: handle short write
	}

	switch s.State {
	case StateDone:
		t.close(s.Fd)
		t.sess.Delete(s.Fd)
		return
	}
}

func (t *Thread) close(fd int) {
	unix.Close(fd)
	t.E.Del(fd, nil)
}
