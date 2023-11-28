package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"projekt/core/cmd/base"
	"projekt/core/lib/local"
	"projekt/core/lib/protocol"
	"projekt/core/lib/relay"
	"projekt/core/lib/secure"
	"projekt/core/lib/session"
	"projekt/core/lib/session/transfer"
	"projekt/core/lib/session/transfer/block"
	"projekt/core/lib/util/buffer"
)

func init() {
	log.SetFlags(log.Ltime)
}

func main() {
	seed := flag.Int64("seed", 0, "seed for predictable keys")
	serverAddr := flag.String("server", "", "the server address (host and port)")
	flag.Parse()

	keyPair, hmacKey := base.GenerateKeys(base.PredictableRandom(*seed))

	inConn := make(chan secure.Conn)

	if serverAddr != nil {
		serverHost, serverPort, err := net.SplitHostPort(*serverAddr)
		if err != nil {
			log.Fatalln("failed to split server address into host and port:", err)
		}
		client, err := relay.Dial(keyPair, serverHost, serverPort)
		if err != nil {
			log.Fatalln("failed to dial server:", err)
		}
		go func() {
			log.Println("accepting connections over the server...")
			conn, err := client.Accept(&base.TrustAny{})
			if err != nil {
				log.Fatalln("failed to accept server connection:", err)
			}
			log.Println("Connection established (server). Receiving file...")
			inConn <- conn
		}()
	}

	client := local.NewClient(base.Port, keyPair, hmacKey, &base.TrustAny{}, base.CipherSuite)
	err := client.Listen()
	if err != nil {
		log.Fatalln("failed to listen:", err)
	}
	go func() {
		log.Println("accepting connections on the local network...")
		conn, err := client.Accept()
		if err != nil {
			log.Fatalln("failed to accept connection:", err)
		}
		log.Println("Connection established (local). Receiving file...")
		inConn <- conn
	}()

	conn := <-inConn
	log.Println("X25519-IDENTITY", base.ToHex(conn.RemoteIdentity()))

	sess := session.NewSession(conn, false)
	channel, err := sess.AcceptChannel(transfer.Protocol{})
	if err != nil {
		log.Fatalln("failed to accept channel:", err)
	}

	var metadata *transfer.Metadata
	var data []byte

	receiver := transfer.NewReceiver(
		func(meta *transfer.Metadata, size uint64) (block.WriteSeekCloser, error) {
			metadata = meta
			data = make([]byte, size)
			buf := buffer.NewBuffer(data)
			return buf, nil
		},
	)

	loop := protocol.NewLoop(receiver.Client, channel)
	err = loop.Run()
	if err != nil {
		log.Fatalln("failed to execute protocol:", err)
	}

	fmt.Println("The file has been successfully received:", metadata.Filename)
	fmt.Println("Data:", string(data))

	err = sess.Close()
	if err != nil {
		log.Fatalln("failed to close session:", err)
	}
}
