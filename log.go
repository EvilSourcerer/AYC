package main

import (
	"io"
	"log"
	"os"
)

func init() {
	f, err := os.OpenFile("exchange.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
		panic("=(")
	}
	log.SetOutput(io.MultiWriter(os.Stdout, f))
}
