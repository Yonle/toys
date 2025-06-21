package main

import (
	"log"
)

func main() {
	var (
		L_ADDR  = "[::1]:1111"
		BUFSIZE = 4096
	)

	t, err := MakeThread(L_ADDR, BUFSIZE)

	if err != nil {
		log.Fatalln("failed to make thread:", err)
	}

	log.Println("Now listening on", L_ADDR)

	go t.HandleBroadcast()

	err = t.StartWaiting()

	if err != nil {
		log.Fatalln("failed to run:", err)
	}
}
