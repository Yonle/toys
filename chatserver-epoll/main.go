package main

import (
	"log"
)

func main() {
	var (
		L_ADDR  = "[::1]:1111"
		BUFSIZE = 4096
	)

	t, err := MakeThread(BUFSIZE)

	if err != nil {
		log.Fatalln("failed to make thread:", err)
	}

	err = t.Listen(L_ADDR)

	if err != nil {
		log.Fatalln("failed to listen:", err)
	}

	log.Println("Now listening on", L_ADDR)

	go t.HandleBroadcast()

	err = t.StartWaiting()

	if err != nil {
		log.Fatalln("failed to run:", err)
	}
}
