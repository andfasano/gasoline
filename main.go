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
)

func main() {

	if len(os.Args) < 3 {
		log.Println("usage: gasoline <cmd> <options>")
		log.Println("available commands:")
		log.Println("  update-ignition-config <source iso> <file> [dest iso]")
	}

	switch os.Args[1] {
	case cmdUpdateIgnitionConfig:
		runUpdateIgnitionConfig(os.Args[1:])
	default:
		log.Fatalf("unknown command")
	}
}

func runUpdateIgnitionConfig(args []string) {
	if len(args) < 3 {
		log.Println("usage: gasoline <source iso> <file> [dest iso]")
		os.Exit(1)
	}

	sourceIso := args[1]
	addFile := args[2]

	destIsoPath := filepath.Join(filepath.Dir(sourceIso), fmt.Sprintf("new-%s", filepath.Base(sourceIso)))
	if len(args) > 3 {
		destIsoPath = args[3]
	}

	keepWorkDir := false

	err := commands.AddFileToIgnitionConfig(sourceIso, addFile, destIsoPath, keepWorkDir)
	if err != nil {
		log.Fatal(err)
	}
}
