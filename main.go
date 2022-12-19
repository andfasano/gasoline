package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 3 {
		log.Println("usage: gasoline <source iso> <file> [dest iso]")
		os.Exit(1)
	}

	sourceIso := os.Args[1]
	addFile := os.Args[2]

	destIsoPath := filepath.Join(filepath.Dir(sourceIso), fmt.Sprintf("new-%s", filepath.Base(sourceIso)))
	if len(os.Args) > 3 {
		destIsoPath = os.Args[3]
	}

	keepWorkDir := false

	err := cmdAddBinaryToIgnitionImage(sourceIso, addFile, destIsoPath, keepWorkDir)
	if err != nil {
		log.Fatal(err)
	}
}

// Method #1: adds the binary file to the ignition image contained within the iso
func cmdAddBinaryToIgnitionImage(sourceIso string, addFile string, destIsoPath string, keepWorkDir bool) error {
	tmpPath, err := os.MkdirTemp("", "gasoline")
	if err != nil {
		return err
	}
	if !keepWorkDir {
		defer os.RemoveAll(tmpPath)
	}

	// Step 1: Unpack the source ISO and ignition.img in a temp folder
	err = isoUnpack(tmpPath, sourceIso)
	if err != nil {
		return err
	}

	err = imgUnpackIgnition(tmpPath)
	if err != nil {
		return err
	}

	// Step 2: Add the binary file to a new ignition config file,
	//         and generate a new compressed cpio archive
	err = addFileToIgnitionConfig(tmpPath, addFile)
	if err != nil {
		return err
	}

	err = imgRepackIgnition(tmpPath)
	if err != nil {
		return err
	}

	err = overwriteOldIgnitionImage(tmpPath)
	if err != nil {
		return err
	}

	// Step 3: Generate a new bootable iso
	err = createBootableIso(tmpPath, destIsoPath)
	if err != nil {
		return err
	}

	return nil
}
