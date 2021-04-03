package main

import (
	"log"

	shortener "github.com/Kh4n/url-shortener-unity/go"
)

func main() {
	server, err := shortener.NewCacheServer("localhost:11211", "http://localhost:8080", 8081, 100)
	if err != nil {
		log.Fatalf("Error starting cache server: %s\n", err.Error())
	}
	log.Fatal(server.Start())
}
