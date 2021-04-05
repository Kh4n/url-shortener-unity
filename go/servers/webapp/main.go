package main

import (
	"flag"
	"log"

	shortener "github.com/Kh4n/url-shortener-unity/go"
)

func main() {
	port := flag.Int("port", 8080, "the port to run the server on")
	if *port < 0 {
		log.Fatalf("Port must be >= 0")
	}
	backendServerHost := flag.String(
		"backendServerHost", "localhost:8081", "the host of the backend server (cache or db)",
	)
	webDir := flag.String(
		"webDir", "./web", "location of web directory",
	)
	flag.Parse()
	server, err := shortener.NewWebappServer(*webDir, *backendServerHost)
	if err != nil {
		log.Fatalf("Error starting server: %s\n", err.Error())
	}
	log.Fatal(server.Start(uint(*port)))
}
