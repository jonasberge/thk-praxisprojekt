package main

import (
	"bufio"
	"fmt"
	"github.com/stoewer/go-strcase"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

//go:generate go run generate.go

const (
	ProtoCompiler   = "protoc"
	ProtoExt        = ".proto"
	ProtoGenExt     = ".pb.go"
	ProtoGenTestExt = ".pb_test.go"
	TestSuffixSep   = "_"
	TestSuffix      = TestSuffixSep + "test"
	LibDir          = "../lib"
)

const (
	GoFormat    = "gofmt"
	EnumPattern = "^\t([A-Z][aA-zZ]*?_+([A-Z][A-Z_]+))"
)

type Test struct {
	Dir  string
	Name string
}

func main() {
	var source []string
	var tests []Test

	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		ext := filepath.Ext(entry.Name())
		if ext == ProtoExt {
			source = append(source, path)
			name := strings.TrimSuffix(entry.Name(), ext)
			if strings.HasSuffix(name, TestSuffix) {
				tests = append(tests, Test{
					Dir:  filepath.Dir(path),
					Name: strings.TrimSuffix(name, TestSuffix),
				})
			}
		}
		return nil
	})
	if err != nil {
		log.Fatal("Failed to collect proto files", err)
	}

	args := []string{
		"--go_opt=module=" + "projekt/core/lib",
		"--go_out=" + LibDir,
		"--go_opt=paths=import",
		"--go-grpc_out=" + LibDir,
		"--go-grpc_opt=paths=import",
	}
	args = append(args, source...)

	// Compile protocol buffers.
	cmd := CreateCommand(ProtoCompiler, args...)
	if err := cmd.Run(); err != nil {
		log.Fatal("Failed to generate", source, err)
	}

	// Replace enumeration values with more concise names.
	// TODO: Is this possible with a plugin?
	for _, path := range source {
		path = filepath.Join(LibDir, path)
		path = strings.TrimSuffix(path, ProtoExt)
		path = path + ProtoGenExt

		for _, replacement := range GetEnumRewrites(path) {
			// One rewrite per command is necessary
			// https://github.com/golang/go/issues/8597
			cmd := CreateCommand(GoFormat, "-w", "-r", replacement.Gofmt(), path)
			if err := cmd.Run(); err != nil {
				log.Fatal("Failed to fix enumerations", err)
			}
		}
	}

	// Rename test files to properly scope test messages.
	for _, path := range tests {
		dir := filepath.Join(LibDir, path.Dir)
		src := filepath.Join(dir, path.Name+TestSuffix+ProtoGenExt)
		dst := filepath.Join(dir, path.Name+ProtoGenTestExt)
		err := os.Rename(src, dst)
		if err != nil {
			log.Fatal("Failed to rename", src, err)
		}
	}
}

func CreateCommand(command string, args ...string) (cmd *exec.Cmd) {
	cmd = exec.Command(command, args...)
	fmt.Println(cmd.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return
}

type Rewrite struct {
	from string
	to   string
}

func (r Rewrite) Gofmt() string {
	return fmt.Sprintf("%v -> %v", r.from, r.to)
}

func GetEnumRewrites(path string) (result []Rewrite) {
	file, err := os.Open(path)
	if err != nil {
		log.Fatal("Failed to open file. Is the go_package option correct?", err)
	}

	regex, err := regexp.Compile(EnumPattern)
	if err != nil {
		log.Fatal("Failed to compile regex", err)
	}

	inConst := false

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := scanner.Text()
		switch text {
		case "const (":
			inConst = true
		case ")":
			inConst = false
		}
		if !inConst || !regex.MatchString(text) {
			continue
		}
		matches := regex.FindStringSubmatch(text)
		result = append(result, Rewrite{
			from: matches[1],
			to:   strcase.UpperCamelCase(matches[2]),
		})
	}

	err = file.Close()
	if err != nil {
		log.Fatal("Failed to close file")
	}
	return
}
