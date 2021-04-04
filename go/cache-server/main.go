package main

import (
	"flag"
	"log"

	shortener "github.com/Kh4n/url-shortener-unity/go"
)

func main() {
	memcacheAddr := flag.String(
		"memcacheAddr", "localhost:11211", "the address of the memcached instance",
	)
	mainServerAddr := flag.String(
		"mainServerAddr", "http://localhost:8080", "the address of the main server",
	)
	port := flag.Int(
		"port", 8081, "port to run this server on. defaults to 8081",
	)
	reserveAmt := flag.Int(
		"reserveAmt", 100, "the number of keys this cache server will reserve from the main server",
	)
	flag.Parse()
	if *port < 0 {
		log.Fatalf("Port must be >= 0")
	}
	if *reserveAmt <= 0 {
		log.Fatalf("Reserve amount must be > 0")
	}

	server, err := shortener.NewCacheServer(
		*memcacheAddr, *mainServerAddr, uint32(*port), uint32(*reserveAmt),
	)
	if err != nil {
		log.Fatalf("Error starting cache server: %s\n", err.Error())
	}
	log.Fatal(server.Start())
}
