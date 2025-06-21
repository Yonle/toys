package main

import (
	"golang.org/x/sys/unix"
)

type Epoll struct {
	Fd int
}

// Make a new epoll instance
func NewEpoll() (e *Epoll, err error) {
	e = &Epoll{}
	e.Fd, err = unix.EpollCreate1(0)
	return
}

// Add en entry to the interest list of the epoll file descriptor
func (e *Epoll) Add(fd int, ev *unix.EpollEvent) (err error) {
	err = e.Ctl(unix.EPOLL_CTL_ADD, fd, ev)
	return
}

// Change the settings associated with `fd` in the interest list to the new settings specified in `unix.EpollEvent`
func (e *Epoll) Mod(fd int, ev *unix.EpollEvent) (err error) {
	err = e.Ctl(unix.EPOLL_CTL_MOD, fd, ev)
	return
}

// Remove (deregister the target file descriptor `fd` from the interest list.
func (e *Epoll) Del(fd int, ev *unix.EpollEvent) (err error) {
	err = e.Ctl(unix.EPOLL_CTL_DEL, fd, ev)
	return
}

// Control interface for an epoll file descriptor from the interest list. The `ev` event argument is ignored and can be `nil`.
func (e *Epoll) Ctl(op, fd int, ev *unix.EpollEvent) (err error) {
	err = unix.EpollCtl(e.Fd, op, fd, ev)
	return
}

// Wait for an I/O event on an epoll file descriptor
func (e *Epoll) Wait(events []unix.EpollEvent, timeout int) (n int, err error) {
	n, err = unix.EpollWait(e.Fd, events, timeout)
	return
}

// Make an object of EpollEvent
func MakeEvent(fd int, events uint32) (ev *unix.EpollEvent) {
	ev = &unix.EpollEvent{
		Events: events,
		Fd:     int32(fd),
	}
	return
}
