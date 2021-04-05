package main

import (
	"flag"
	"log"

	shortener "github.com/Kh4n/url-shortener-unity/go"
)

func main() {
	dbPath := flag.String(
		"dbPath", "./badger-db", "path to database",
	)
	port := flag.Int("port", 8082, "the port to run the server on")
	flag.Parse()
	if *port < 0 {
		log.Fatalf("Port must be >= 0")
	}
	server, err := shortener.NewMainServer(*dbPath)
	if err != nil {
		log.Fatalf("Error starting server: %s\n", err.Error())
	}
	log.Fatal(server.Start(uint(*port)))
}
