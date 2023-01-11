package commands

import (
	"os"

	"github.com/andfasano/gasoline/pkg/gasoline"
)

// Method #2: appends the binary file to the ignition image contained within the iso
func AppendFileToIgnitionImg(sourceIso string, addFile string, destIsoPath string, keepWorkDir bool) error {
	tmpPath, err := os.MkdirTemp("", "gasoline")
	if err != nil {
		return err
	}
	if !keepWorkDir {
		defer os.RemoveAll(tmpPath)
	}

	// Step 1: Unpack the source ISO and ignition.img in a temp folder
	err = gasoline.IsoUnpack(tmpPath, sourceIso)
	if err != nil {
		return err
	}

	// Step 2: Append file and create a new ignition.img
	err = gasoline.ImgIgnitionAppend(tmpPath, addFile)
	if err != nil {
		return err
	}

	// Step 3: Generate a new bootable iso using the new ignition.img
	err = gasoline.OverwriteOldIgnitionImage(tmpPath)
	if err != nil {
		return err
	}

	err = gasoline.CreateBootableIso(tmpPath, destIsoPath)
	if err != nil {
		return err
	}

	return nil
}
