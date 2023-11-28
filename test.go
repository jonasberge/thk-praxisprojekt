package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
)

// NOTE There is a bug where test synchronization will deadlock.
//  This doesn't happen too frequently though.

// docker-compose -f "$COMPOSE" exec -T -- alpha go test -v -tags=container -count=1 "./lib/$PACKAGE"
// docker-compose -f "$COMPOSE" exec -T -- bravo go test -v -tags=container -count=1 "./lib/$PACKAGE"

const Command = "docker-compose"

var containers = []string{"alpha", "bravo"}

func main() {
	args := os.Args[1:]
	if len(args) != 1 && len(args) != 2 {
		log.Fatalln("need one or two arguments: name of package to test + optional method to run")
	}
	pkg := args[0]
	rn := ""
	if len(args) == 2 {
		rn = args[1]
	}

	// go does not kill processes started with exec.Cmd.Run
	// so we will have to do it manually before every run.
	for _, container := range containers {
		cmd := exec.Command(Command, getKillArgs(container)...)
		_ = cmd.Run() // ignore the error as pkill will exit with 1 if there is no process to kill...
	}

	writers := make([]io.Writer, 0, len(containers))
	buffers := make([]*bytes.Buffer, 0, len(containers)-1)
	for i := 0; i < len(containers); i++ {
		var writer io.Writer = os.Stdout
		if i > 0 {
			buffer := &bytes.Buffer{}
			buffers = append(buffers, buffer)
			writer = buffer
		}
		writers = append(writers, writer)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		//writeOutputs(buffers)
	}()

	var wg sync.WaitGroup
	for i, container := range containers {
		cmd := run(container, pkg, rn, writers[i])
		prefix := "\n"
		if i == 0 {
			prefix = ""
		}
		title := fmt.Sprintf("%v+ %v\n", prefix, cmd.String())
		_, err := writers[i].Write([]byte(title))
		if err != nil {
			log.Fatalln("failed to write title:", err)
		}
		err = cmd.Start()
		if err != nil {
			log.Fatalln("failed to run command:", err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			e := cmd.Wait()
			if e != nil {
				log.Println("ERROR failed to wait for command to finish")
			}
		}()
	}
	wg.Wait()
	writeOutputs(buffers)
}

func writeOutputs(buffers []*bytes.Buffer) {
	for _, buf := range buffers {
		_, err := os.Stdout.Write(buf.Bytes())
		if err != nil {
			log.Fatalln("failed to write command output")
		}
	}
}

func run(container string, pkg string, run string, out io.Writer) *exec.Cmd {
	args := getRunArgs(container, pkg, run)
	cmd := exec.Command(Command, args...)
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd
}

func getRunArgs(container string, pkg string, run string) []string {
	res := append(getBaseArgs(), []string{
		container,
		"go",
		"test",
		"-v",
		"-tags=container",
		"-count=1",
		"./lib/" + pkg,
	}...)
	if run != "" {
		res = append(res, "-run")
		res = append(res, run)
	}
	return res
}

func getKillArgs(container string) []string {
	return append(getBaseArgs(), []string{
		container,
		"pkill",
		"go",
	}...)
}

func getBaseArgs() []string {
	return []string{
		"-f",
		"docker-compose.test.yml",
		"exec",
		"-T",
		"--",
	}
}
