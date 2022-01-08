package main

import (
	"fmt"
	"os"
	"os/signal"
	"partage-ca/server/impl"
	"syscall"
)

func main() {
	stop := make(chan os.Signal, 2)
	signal.Notify(stop, os.Interrupt, syscall.SIGINT)

	server := impl.NewServer()
	fmt.Println("starting CA server...")
	server.Start()

	<-stop //blocks until SIGINT (ctrl+c) signal is received
	server.Stop()
}
