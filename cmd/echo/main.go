package main

import (
	"context"
	"flag"
	"github.com/gofu/echo"
	"log"
	"os"
	"os/signal"
	"time"
)

func main() {
	addr := flag.String("addr", "0.0.0.0:3246", "HTTP address to listen on")
	flag.Parse()
	ctx, cancel := context.WithCancel(context.Background())
	mainCh := make(chan error, 1)
	go func() { mainCh <- echo.Main(ctx, *addr) }()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	var timeoutCh <-chan time.Time
	for {
		select {
		case <-sigCh:
			cancel()
			log.Println("Shutting down")
			timeoutCh = time.After(10 * time.Second)
		case <-timeoutCh:
			log.Fatalln("Shutdown timed out")
		case err := <-mainCh:
			if err != context.Canceled {
				log.Fatal(err)
			}
			log.Println("Shut down successfully")
			return
		}
	}
}
