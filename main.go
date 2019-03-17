package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	SetupDatabase() // not in a goroutine because this just sets up a connection, it doesn't block
	createInitialListings()
	setupDiscordBot()
	go slotExpiries()
	go pendingDepositCleanup()
	go pendingWithdrawalCleanup()
	go baritoneListen()
	go serve()

	awaitControlC() // not in a goroutine because this is intended to wait until it's time to shut down
	log.Println("Goodbye")
	// Go programs exit once the main function is over, so once "awaitControlC" returns, the program will quit immediately
}

func awaitControlC() {
	// when you press control+c normally it just kills the process instantly
	// we cannot have that, since it could mess up the database
	// Go allows us to capture these signals, and do some cleanup before actually quitting

	sigs := make(chan os.Signal, 1)                      // a channel of signals
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM) // now when we receive SIGINT or SIGTERM (control+c in terminal), it will just send the signal on this channel instead of quitting

	sig := <-sigs // receive a signal from the sigs channel. this will block until a signal actually comes through.

	// at this point, we've just received a signal that's either SIGINT or SIGTERM so time to shut down

	log.Println("Control C pressed, shutting down cleanly")
	log.Println("Signal received:", sig)

	log.Println("Shutting down HTTP server")
	ShutdownHTTP()

	log.Println("Shutting down database")
	ShutdownDatabase()

	log.Println("Everything is shut down")
	//time to shut down for real, everything is cleaned up already
}
