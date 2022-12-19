package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/cavaliergopher/cpio"
	ignutil "github.com/coreos/ignition/v2/config/util"
	igntypes "github.com/coreos/ignition/v2/config/v3_2/types"
	"github.com/vincent-petithory/dataurl"
)

const (
	ignitionImage     string = "ignition.img"
	ignitionImagePath string = "images/ignition.img"

	ignitionFile        string = "config.ign"
	sourceIgnitionFile  string = "source-config.ign"
	updatedIgnitionFile string = "config.ign"

	binaryDestFolder string = "/usr/local/bin"
)

// imgUnpackIgnition extracts the config.ign file contained within the
// compressed cpio archive and saves it into the temp folder with a new name
func imgUnpackIgnition(tmpPath string) error {
	ignitionImgPath := filepath.Join(tmpPath, isoTempSubFolder, ignitionImagePath)

	log.Println("Unpacking ignition image...")

	f, err := os.Open(ignitionImgPath)
	if err != nil {
		return err
	}
	defer f.Close()

	g, err := gzip.NewReader(f)
	if err != nil {
		return err
	}

	r := cpio.NewReader(g)
	if err != nil {
		return err
	}

	for {
		hdr, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if hdr.Name != ignitionFile {
			return fmt.Errorf("invalid ignition file found: %s", hdr.Name)
		}

		dest, err := os.Create(filepath.Join(tmpPath, sourceIgnitionFile))
		if err != nil {
			return err
		}
		defer dest.Close()

		_, err = io.Copy(dest, r)
		if err != nil {
			return err
		}
	}

	return nil
}

// Adds a binary file to the source ignition file, ad save it
// using a default name
func addFileToIgnitionConfig(tmpPath string, addFile string) error {
	sourceIgnition := filepath.Join(tmpPath, sourceIgnitionFile)
	destIgnition := filepath.Join(tmpPath, updatedIgnitionFile)

	log.Printf("Adding %s to %s\n", addFile, destIgnition)

	f, err := os.ReadFile(sourceIgnition)
	if err != nil {
		return err
	}

	var config igntypes.Config
	err = json.Unmarshal(f, &config)
	if err != nil {
		return err
	}

	// Grab the new file to be added to ignition
	newFileData, err := os.ReadFile(addFile)
	if err != nil {
		return err
	}

	config.Storage.Files = append(config.Storage.Files, igntypes.File{
		Node: igntypes.Node{
			Group:     igntypes.NodeGroup{},
			Overwrite: ignutil.BoolToPtr(true),
			Path:      filepath.Join(binaryDestFolder, filepath.Base(addFile)),
			User: igntypes.NodeUser{
				Name: ignutil.StrToPtr("root"),
			},
		},
		FileEmbedded1: igntypes.FileEmbedded1{
			Contents: igntypes.Resource{
				Source: ignutil.StrToPtr(dataurl.EncodeBytes(newFileData)),
			},
			Mode: ignutil.IntToPtr(0755),
		},
	})

	newIgnition, err := json.Marshal(config)
	if err != nil {
		return err
	}
	err = os.WriteFile(destIgnition, newIgnition, 0777)
	if err != nil {
		return err
	}

	return nil
}

// Creats a new ignition.img containing an updated ignition file
func imgRepackIgnition(tmpPath string) error {
	configFilePath := filepath.Join(tmpPath, updatedIgnitionFile)
	newIgnitionImgPath := filepath.Join(tmpPath, ignitionImage)

	log.Println("Repacking new ignition.img with the updated config.ign file")

	f, err := os.Create(newIgnitionImgPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	w := cpio.NewWriter(gw)
	defer w.Close()

	// add config file to the archive
	cf, err := os.Open(configFilePath)
	if err != nil {
		return err
	}
	defer cf.Close()

	info, err := cf.Stat()
	if err != nil {
		return err
	}
	log.Printf("New config.ign size: %dK", info.Size()/1024)

	hdr, err := cpio.FileInfoHeader(info, "")
	if err != nil {
		return err
	}

	if err = w.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := io.Copy(w, cf); err != nil {
		return err
	}

	return nil
}

// Overwrites the old ignition image with the new one
func overwriteOldIgnitionImage(tmpPath string) error {
	// Copy the updated ignition archive in the temp folder
	srcFile := filepath.Join(tmpPath, ignitionImage)
	dstFile := filepath.Join(tmpPath, isoTempSubFolder, ignitionImagePath)

	srcStat, err := os.Stat(srcFile)
	if err != nil {
		return err
	}
	dstState, err := os.Stat(dstFile)
	if err != nil {
		return err
	}
	log.Printf("New ignition.img size: %dK (old was %dK)", srcStat.Size()/1024, dstState.Size()/1024)

	err = os.Rename(srcFile, dstFile)
	if err != nil {
		return err
	}

	return nil
}
