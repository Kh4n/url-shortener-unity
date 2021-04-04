package main

import (
	"flag"
	"log"

	shortener "github.com/Kh4n/url-shortener-unity/go"
)

func main() {
	port := flag.Int("port", 8080, "the port to run the server on. defaults to 8080")
	flag.Parse()
	if *port < 0 {
		log.Fatalf("Port must be >= 0")
	}
	server, err := shortener.NewMainServer("./db", uint32(*port))
	if err != nil {
		log.Fatalf("Error starting server: %s\n", err.Error())
	}
	log.Fatal(server.Start())
}
