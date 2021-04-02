package main

import "log"

func main() {
	server := NewCacheServer("localhost:11211", "http://localhost:8080", 8081, 100)
	log.Fatal(server.Start())
}
