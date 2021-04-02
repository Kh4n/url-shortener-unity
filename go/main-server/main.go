package main

import (
	"log"
)

func main() {
	server, err := NewMainServer("./db", 8080)
	if err != nil {
		log.Fatalf("Error starting server: %s\n", err.Error())
	}
	log.Fatal(server.Start())
}
