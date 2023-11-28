package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"projekt/core/cmd/base"
	"projekt/core/lib/local"
	"projekt/core/lib/protocol"
	"projekt/core/lib/relay"
	"projekt/core/lib/session"
	"projekt/core/lib/session/transfer"
	"projekt/core/lib/session/transfer/block"
	"time"
)

func init() {
	log.SetFlags(log.Ltime)
}

func main() {
	argSeed := flag.Int64("seed", 1, "seed for predictable keys")
	argIdentity := flag.String("identity", "", "public key of the target device")
	argKey := flag.String("key", "", "device key of the target device for local connections")
	argServer := flag.String("server", "", "the server address (host and port)")
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatalln("expected a file name argument")
	}
	argFilePath := flag.Args()[0]

	targetIdentity := base.DecodeHex(*argIdentity, base.IdentitySize)
	targetHmacKey := base.DecodeHex(*argKey, base.DeviceKeySize)

	file, err := os.Open(argFilePath)
	if err != nil {
		log.Fatalf("failed to open file \"%s\": %v\n", argFilePath, err)
	}
	fileInfo, err := file.Stat()
	if err != nil {
		log.Fatalln("failed to stat file:", err)
	}

	keyPair, hmacKey := base.GenerateKeys(base.PredictableRandom(*argSeed))
	client := local.NewClient(base.Port, keyPair, hmacKey, &base.TrustNone{}, base.CipherSuite)
	err = client.Listen()
	if err != nil {
		log.Fatalln("failed to listen:", err)
	}
	defer func() {
		e1 := client.Close()
		if e1 != nil {
			log.Fatalln("failed to close client:", e1)
		}
	}()

	const Timeout = 1 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	log.Printf("Attempting to contact device locally with a timeout of %v...\n", Timeout.String())
	conn, err := client.Connect(ctx, targetIdentity, targetHmacKey)
	if err != nil {
		log.Println("failed to connect to device:", err)
		if argServer == nil {
			os.Exit(1)
		}

		serverHost, serverPort, err := net.SplitHostPort(*argServer)
		if err != nil {
			log.Fatalln("failed to split server address into host and port:", err)
		}
		client, err := relay.Dial(keyPair, serverHost, serverPort)
		if err != nil {
			log.Fatalln("failed to dial server:", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), base.Timeout)
		defer cancel()
		log.Printf("Attempting to contact device over the server with a timeout of %v...\n", base.Timeout.String())
		conn, err = client.Request(ctx, targetIdentity)
		if err != nil {
			log.Fatalln("failed to connect to device:", err)
		}
	}
	defer conn.Close()

	log.Println("Connection established. Transferring file...")
	log.Println("RECEIVER-IDENTITY", base.ToHex(conn.RemoteIdentity()))

	sess := session.NewSession(conn, true)
	channel, err := sess.OpenChannel(transfer.Protocol{})
	if err != nil {
		log.Fatalln("failed to open channel:", err)
	}

	metadata := &transfer.Metadata{Filename: filepath.Base(file.Name())}
	reader := block.NewBlockReader(file, uint64(fileInfo.Size()), base.BlockSize)
	sender := transfer.NewSender(metadata, reader)

	loop := protocol.NewLoop(sender.Client, channel)
	err = loop.Run()
	if err != nil {
		log.Fatalln("failed to execute protocol:", err)
	}

	fmt.Println("The file has been successfully transmitted.")

	err = sess.Close()
	if err != nil {
		log.Fatalln("failed to close session:", err)
	}
}
