package commands

import (
	"os"

	"github.com/andfasano/gasoline/pkg/gasoline"
)

// Method #1: adds the binary file to the ignition image contained within the iso
func AddFileToIgnitionConfig(sourceIso string, addFile string, destIsoPath string, keepWorkDir bool) error {
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

	err = gasoline.ImgUnpackIgnition(tmpPath)
	if err != nil {
		return err
	}

	// Step 2: Add the binary file to a new ignition config file,
	//         and generate a new compressed cpio archive
	err = gasoline.AddFileToIgnitionConfig(tmpPath, addFile)
	if err != nil {
		return err
	}

	err = gasoline.ImgRepackIgnition(tmpPath)
	if err != nil {
		return err
	}

	err = gasoline.OverwriteOldIgnitionImage(tmpPath)
	if err != nil {
		return err
	}

	// Step 3: Generate a new bootable iso
	err = gasoline.CreateBootableIso(tmpPath, destIsoPath)
	if err != nil {
		return err
	}

	return nil
}
