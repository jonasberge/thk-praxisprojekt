package main

import (
	"crypto/tls"
	"flag"
	"log"
	"path/filepath"
	"projekt/core/lib/relay"
)

func init() {
	log.SetFlags(log.Ltime)
}

func main() {
	port := flag.Int("port", 23520, "server port")
	sessionPort := flag.Int("sessionPort", 23521, "session port")
	dir := flag.String("keyDir", "", "directory of the server crt and key")
	flag.Parse()

	if *dir == "" {
		log.Fatalln("server crt and key directory is required")
	}
	if *port == 0 {
		log.Fatalln("server port cannot be 0")
	}
	if *sessionPort == 0 {
		log.Fatalln("session port cannot be 0")
	}

	cert, _ := tls.LoadX509KeyPair(filepath.Join(*dir, "server.crt"), filepath.Join(*dir, "server.key"))
	server := relay.NewServer(cert, *port, *sessionPort)
	err := server.Listen()
	if err != nil {
		log.Fatalln("failed to listen:", err)
	}

	log.Printf("listening on ports %v and %v\n", *port, *sessionPort)
	select {}
}
