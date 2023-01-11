package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/andfasano/gasoline/pkg/commands"
)

const (
	cmdUpdateIgnitionConfig = "update-ignition-config"
	cmdAppendIgnitionImg    = "append-ignition-image"
)

func main() {

	if len(os.Args) < 3 {
		log.Println("usage: gasoline <cmd> <options>")
		log.Println("available commands:")
		log.Printf("  %s <source iso> <file> [dest iso]\n", cmdUpdateIgnitionConfig)
		log.Printf("  %s <source iso> <file> [dest iso]\n", cmdAppendIgnitionImg)
	}

	cmd := os.Args[1]
	cmdArgs := os.Args[1:]

	switch cmd {
	case cmdUpdateIgnitionConfig:
		runUpdateIgnitionConfig(cmdArgs)
	case cmdAppendIgnitionImg:
		runAppendIngitionImg(cmdArgs)

	default:
		log.Fatalf("unknown command")
	}
}

func runUpdateIgnitionConfig(args []string) {
	if len(args) < 3 {
		log.Printf("usage: gasoline %s <source iso> <file> [dest iso]\n", cmdUpdateIgnitionConfig)
		os.Exit(1)
	}

	sourceIso := args[1]
	addFile := args[2]

	destIsoPath := filepath.Join(filepath.Dir(sourceIso), fmt.Sprintf("new-%s", filepath.Base(sourceIso)))
	if len(args) > 3 {
		destIsoPath = args[3]
	}

	err := commands.AddFileToIgnitionConfig(sourceIso, addFile, destIsoPath, false)
	if err != nil {
		log.Fatal(err)
	}
}

func runAppendIngitionImg(args []string) {
	if len(args) < 3 {
		log.Printf("usage: gasoline %s <source iso> <file> [dest iso]\n", cmdAppendIgnitionImg)
		os.Exit(1)
	}

	sourceIso := args[1]
	addFile := args[2]

	destIsoPath := filepath.Join(filepath.Dir(sourceIso), fmt.Sprintf("new-%s", filepath.Base(sourceIso)))
	if len(args) > 3 {
		destIsoPath = args[3]
	}

	err := commands.AppendFileToIgnitionImg(sourceIso, addFile, destIsoPath, true)
	if err != nil {
		log.Fatal(err)
	}
}
