package main

import (
	"log"
	"sync"
)

func main() {
	var (
		L_ADDR  = "[::1]:1111"
		BUFSIZE = 4096
	)

	wg := &sync.WaitGroup{}

	t := 4
	// in go, this way doesn't make any sense.
	// but again, since we're doing it in low level, that may makes sense

	wg.Add(t)
	for i := 0; t > i; i++ {
		go func() {
			defer wg.Done()
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
		}()
	}

	wg.Wait()
}
