package main

import (
	"flag"
	"log"

	shortener "github.com/Kh4n/url-shortener-unity/go"
)

func main() {
	memcachedHost := flag.String(
		"memcachedHost", "localhost:11211", "the host of the memcached instance",
	)
	dbServerHost := flag.String(
		"dbServerHost", "localhost:8082", "the host of the db server",
	)
	port := flag.Int(
		"port", 8081, "port to run this server on",
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
		*memcachedHost, *dbServerHost, uint32(*reserveAmt),
	)
	if err != nil {
		log.Fatalf("Error starting cache server: %s\n", err.Error())
	}
	log.Fatal(server.Start(uint(*port)))
}
