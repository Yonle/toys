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

	pool *sync.Pool

	bw    *sync.Map
	conns *sync.Map

	M chan *Data
}

type Data struct {
	D    []byte
	From int // messenger's fd
}

func MakeThread(BUFSIZE int) (t *Thread, err error) {
	t = &Thread{
		bw:    &sync.Map{},
		conns: &sync.Map{},

		M: make(chan *Data),
		pool: &sync.Pool{
			New: func() any {
				return make([]byte, BUFSIZE)
			},
		},
	}

	t.E, err = epoll.NewInstance(0)
	return
}

func (t *Thread) Listen(L_ADDR string) (err error) {
	t.L, err = makeListener()
	if err != nil {
		return
	}

	t.bw.Store(t.L.Fd, nil)

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
					t.handleNewConn()
				default: // from client
					t.handleClient(fd)
				}
			}

			if e.Events&unix.EPOLLOUT != 0 { // something is ready to be feed
				t.bw.Store(fd, nil)
			}
		}
	}
}

func (t *Thread) handleNewConn() {
	// let's accept new guest!
	nfd, _, err := t.L.Accept()

	if err != nil {
		log.Println("  failed to accept new fd:", err)
		return
	}

	ev := epoll.MakeEvent(nfd, (unix.EPOLLIN | unix.EPOLLRDHUP))
	t.E.Add(nfd, ev)
	log.Printf("  look! new guest! it's fd %d!", nfd)

	t.conns.Store(nfd, nil)
}

func (t *Thread) handleClient(fd int) {
	buf := t.pool.Get().([]byte)
	defer t.pool.Put(buf)

	n, err := unix.Read(fd, buf)

	switch {
	case err != nil:
		t.close(fd)
		return
	}

	if n == 0 {
		t.close(fd)
		return
	}

	t.M <- &Data{D: buf[:n], From: fd} // send to global channel
}

func (t *Thread) close(fd int) {
	unix.Close(fd)

	t.conns.Delete(fd)
	t.bw.Delete(fd)

	t.E.Del(fd, nil)
}

func (t *Thread) HandleBroadcast() {
	for data := range t.M {
		t.broadcast(data)
	}
}

func (t *Thread) broadcast(data *Data) {
	t.conns.Range(func(fd, _ any) bool {
		if _, dont := t.bw.Load(fd); dont || data.From == fd {
			return true
		}

		_, err := unix.Write(fd.(int), data.D)

		switch {
		case err == unix.EPIPE:
			t.bw.Store(fd, nil) // shh. don't talk
			log.Printf("    %d closed read on their end", fd)
		case err != nil:
			t.close(fd.(int))
		}

		return true
	})
}
