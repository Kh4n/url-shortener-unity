package main

import (
	"flag"
	"log"

	shortener "github.com/Kh4n/url-shortener-unity/go"
)

func main() {
	port := flag.Int("port", 8080, "the port to run the server on")
	flag.Parse()
	if *port < 0 {
		log.Fatalf("Port must be >= 0")
	}
	backendServerAddr := flag.String(
		"backendServerAddr", "http://localhost:8081", "the address of the backend server (cache or db)",
	)
	server, err := shortener.NewWebappServer(*backendServerAddr)
	if err != nil {
		log.Fatalf("Error starting server: %s\n", err.Error())
	}
	log.Fatal(server.Start(uint(*port)))
}
