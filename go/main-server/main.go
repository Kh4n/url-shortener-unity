package main

import (
	"log"

	shortener "github.com/Kh4n/url-shortener-unity/go"
)

func main() {
	server, err := shortener.NewMainServer("./db", 8080)
	if err != nil {
		log.Fatalf("Error starting server: %s\n", err.Error())
	}
	log.Fatal(server.Start())
}
