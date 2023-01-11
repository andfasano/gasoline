package gasoline

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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
func ImgUnpackIgnition(tmpPath string) error {
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
func AddFileToIgnitionConfig(tmpPath string, addFile string) error {
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
			Mode: ignutil.IntToPtr(0644),
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
func ImgRepackIgnition(tmpPath string) error {
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
	err = store(w, configFilePath)
	if err != nil {
		return err
	}

	return nil
}

// Overwrites the old ignition image with the new one
func OverwriteOldIgnitionImage(tmpPath string) error {
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

func ImgIgnitionAppend(tmpPath string, addFilePath string) error {

	// This temporary folder will contain all the files to be
	// added in the new ignition.img archive
	tmpIgnitionPath := filepath.Join(tmpPath, "tmp-ignition")
	err := os.Mkdir(tmpIgnitionPath, 0755)
	if err != nil {
		return err
	}

	// Copy the additional file in the temp folder
	input, err := ioutil.ReadFile(addFilePath)
	if err != nil {
		return err
	}

	dstFile := filepath.Join(tmpIgnitionPath, filepath.Base(addFilePath))
	err = ioutil.WriteFile(dstFile, input, 0644)
	if err != nil {
		return err
	}

	// Extract the old ignition.img in the tempo folder
	err = extractCpioFiles(tmpPath, tmpIgnitionPath)
	if err != nil {
		return err
	}

	// Create the new archive
	err = createCpioArchive(tmpPath, tmpIgnitionPath)
	if err != nil {
		return err
	}

	return nil
}

// Extracts the content of the ignition.img in the temp folder
func extractCpioFiles(tmpPath string, dstPath string) error {
	oldIgnitionImgPath := filepath.Join(tmpPath, isoTempSubFolder, ignitionImagePath)

	// Open existing ignition.img
	f, err := os.Open(oldIgnitionImgPath)
	if err != nil {
		return err
	}
	defer f.Close()

	g, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer g.Close()

	cpioReader := cpio.NewReader(g)
	if err != nil {
		return err
	}

	for {
		hdr, err := cpioReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		dstFile, err := os.Create(filepath.Join(dstPath, hdr.Name))
		if err != nil {
			return err
		}
		defer dstFile.Close()

		if _, err := io.Copy(dstFile, cpioReader); err != nil {
			return err
		}
	}

	return nil
}

func createCpioArchive(tmpPath string, srcPath string) error {

	var buf bytes.Buffer

	cpioWriter := cpio.NewWriter(&buf)

	files, err := ioutil.ReadDir(srcPath)
	if err != nil {
		return nil
	}

	for _, f := range files {
		err := store(cpioWriter, filepath.Join(srcPath, f.Name()))
		if err != nil {
			return err
		}
	}

	err = cpioWriter.Close()
	if err != nil {
		return err
	}

	out, err := os.Create(filepath.Join(tmpPath, ignitionImage))
	if err != nil {
		return err
	}
	defer out.Close()

	bufWriter := bufio.NewWriter(out)
	defer bufWriter.Flush()

	gz := gzip.NewWriter(bufWriter)
	defer gz.Close()

	_, err = io.Copy(gz, &buf)
	if err != nil {
		return err
	}

	return nil
}

func store(w *cpio.Writer, fn string) error {
	f, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	hdr, err := cpio.FileInfoHeader(fi, "")
	if err != nil {
		return err
	}
	if err := w.WriteHeader(hdr); err != nil {
		return err
	}
	if !fi.IsDir() {
		if _, err := io.Copy(w, f); err != nil {
			return err
		}
	}
	return err
}
