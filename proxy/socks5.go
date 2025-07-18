package main

// This is where SOCKS5 logic lives.
// Socket creation is on conn.go

import (
	"encoding/binary"
	"log"
	"slices"

	"github.com/Yonle/go-epoll"
	"golang.org/x/sys/unix"
)

type ST int
type Yeet int

const (
	StateInit        = ST(000)
	StateNeedMethod  = ST(100)
	StatePlainAuth   = ST(102)
	StateNeedCmd     = ST(200)
	StateEstablished = ST(255)
	StateDone        = ST(300)
)

const (
	Yeet_Die            = Yeet(1)
	Yeet_TryAgain       = Yeet(2)
	Yeet_InvalidThenDie = Yeet(3)
)

type Destination struct {
	Type byte
	Addr byte
	Port byte
}

type Session struct {
	E      *epoll.Instance
	Fd     int
	DestFD int
	State  ST
	Method byte
	Auth   byte
	Cmd    byte
	Dest   Destination

	rb []byte // read buf
	rl int    // read len
	sb []byte // send buf
	sl int    // send len
}

const (
	Ver = byte(0x05) // Version
	Res = byte(0x00) // Reserved

	Atyp_Inet4 = byte(0x01)
	Atyp_Inet6 = byte(0x04)
	Atyp_Name  = byte(0x03)

	// Client to server
	Auth_NoAuth   = byte(0x00)
	Auth_GssAPI   = byte(0x01)
	Auth_Plain    = byte(0x02)
	Auth_NoMethod = byte(0xFF)
	Cmd_Connect   = byte(0x01)
	Cmd_Bind      = byte(0x02)
	Cmd_UDP       = byte(0x03)

	// Server to client
	Rep_Success         = byte(0x00)
	Rep_Fail            = byte(0x01)
	Rep_Forbidden       = byte(0x02)
	Rep_NetUnreachable  = byte(0x03)
	Rep_HostUnreachable = byte(0x04)
	Rep_ConnRefused     = byte(0x05)
	Rep_TTLexpired      = byte(0x06)
	Rep_UnsupportedCmd  = byte(0x07)
	Rep_UnsupportedAddr = byte(0x08)
)

func (t *Thread) MakeSession(fd int) *Session {
	return &Session{
		E:     t.E,
		Fd:    fd,
		State: StateInit,
		rb:    make([]byte, BUFSIZE),
	}
}

// where yeet = yeet the connection
// yeet -> 1 = Yeet connection
// yeet -> 2 = EAGAIN
// yeet -> 3 = Invalid
func (s *Session) CheckAuth() (yeet Yeet) {
	if s.rl < 2 {
		yeet = Yeet_TryAgain
		return
	}

	d := s.rb[:s.rl]

	if d[0] != Ver {
		yeet = Yeet_Die
		return
	}

	nmethods := d[1]
	if nmethods == 0 {
		yeet = Yeet_Die
		return
	}

	metalen := 2 + int(nmethods)

	if len(d) != metalen {
		yeet = Yeet_TryAgain
		return
	}

	if !slices.Contains(d[2:], Auth_NoAuth) {
		yeet = Yeet_InvalidThenDie
		s.sb = append(s.sb[:0], Ver, Auth_NoMethod)
		s.sl = len(s.sb)
		return
	}

	s.State = StateNeedCmd
	s.sb = append(s.sb[:0], Ver, Auth_NoAuth)
	s.sl = len(s.sb)
	s.rl -= metalen

	if s.rl != 0 {
		moveToFront(s.rb, metalen)
	}

	return
}

func (s *Session) CheckCmd() (yeet Yeet) {
	if s.rl < 4 {
		yeet = Yeet_TryAgain
	}

	d := s.rb[:s.rl]

	if d[0] != Ver {
		yeet = Yeet_Die
		return
	}

	switch d[1] {
	case Cmd_Connect:
		yeet = s.CmdConnect()
	}

	return
}

func (s *Session) CmdConnect() (yeet Yeet) {
	d := s.rb[:s.rl]
	if d[2] != Res {
		yeet = Yeet_InvalidThenDie
		return
	}

	exp_len := 4

	switch d[3] {
	case Atyp_Inet4:
		exp_len += 4 + 2
	case Atyp_Inet6:
		exp_len += 16 + 2
	case Atyp_Name:
		exp_len += 1
		if s.rl < exp_len {
			yeet = Yeet_TryAgain
			return
		}

		exp_len += int(d[4]) + 2
	default:
		// it's invalid
		yeet = Yeet_InvalidThenDie
		return
	}

	if s.rl < exp_len {
		yeet = Yeet_TryAgain
		return
	}

	cut := 4

	if d[3] == Atyp_Name {
		cut += 1
	}

	s.rl -= exp_len
	s.rb = s.rb[:0]

	addrB := d[cut : len(d)-2]
	portB := d[len(d)-2:]

	port := binary.BigEndian.Uint16(portB)

	log.Println("Addr Buf:", addrB)
	log.Println("Port Buf:", portB)
	log.Println("Parsed Port:", port)

	if d[3] == Atyp_Name {
		log.Println("Domain request:", string(addrB))
	}

	fd, err := s.ConnectTo(d[3], addrB, portB)

	if err != nil {
		// do something...
		log.Println("fail to make socket() / connect():", err)
		s.SrvReply(Rep_Fail, fd)
		return
	}

	s.DestFD = fd

	se := epoll.MakeEvent(fd, unix.EPOLLOUT)
	s.E.Add(fd, se)

	return
}

func (s *Session) SrvReply(rep byte, fd int) error {
	bindsa, err := unix.Getsockname(fd)

	if err != nil {
		return err
	}

	var atyp byte
	var ip []byte
	port := make([]byte, 2)

	switch v := bindsa.(type) {
	case *unix.SockaddrInet4:
		atyp = Atyp_Inet4
		ip = v.Addr[:]
		binary.BigEndian.PutUint16(port, uint16(v.Port))
	case *unix.SockaddrInet6:
		atyp = Atyp_Inet6
		ip = v.Addr[:]
		binary.BigEndian.PutUint16(port, uint16(v.Port))
	}

	s.sb = append(s.sb[:0], Ver, rep, Res, atyp)
	s.sb = append(s.sb, ip...)
	s.sb = append(s.sb, port...)
	s.sl = len(s.sb)
	return nil
}

// to client
func (s *Session) Write(b []byte) {
	s.sb = append(s.sb[:0], b...)
	s.sl = len(b)
}
