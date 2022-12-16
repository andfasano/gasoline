package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/vincent-petithory/dataurl"

	"github.com/cavaliergopher/cpio"
	ignutil "github.com/coreos/ignition/v2/config/util"
	igntypes "github.com/coreos/ignition/v2/config/v3_2/types"
	"github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/filesystem/iso9660"
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

	err := repackageIgnitionWith(sourceIso, addFile, destIsoPath, keepWorkDir)
	if err != nil {
		log.Fatal(err)
	}
}

func repackageIgnitionWith(sourceIso string, addFile string, destIsoPath string, keepWorkDir bool) error {
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

	// Step 3: Generate a new bootable iso
	err = makeNewBootableISO(tmpPath, destIsoPath)
	if err != nil {
		return err
	}

	return nil
}

func isoUnpack(tmpPath string, isoPath string) error {
	// Create tmp folder for unpacking the source iso
	tmpIsoPath := filepath.Join(tmpPath, "iso")
	err := os.Mkdir(tmpIsoPath, 0755)
	if err != nil {
		return err
	}

	log.Printf("Unpacking %s in the temp folder %s\n", isoPath, tmpIsoPath)

	disk, err := diskfs.OpenWithMode(isoPath, diskfs.ReadOnly)
	if err != nil {
		return err
	}
	fs, err := disk.GetFilesystem(0)
	if err != nil {
		return err
	}
	err = isoExtractAll(tmpIsoPath, "/", fs)
	if err != nil {
		return err
	}

	return nil
}

func isoExtractAll(dstPath string, path string, fs filesystem.FileSystem) error {
	files, err := fs.ReadDir(path)
	if err != nil {
		return err
	}
	for _, file := range files {

		fullPath := filepath.Join(path, file.Name())

		// Create the directory
		if file.IsDir() {
			err = os.Mkdir(filepath.Join(dstPath, fullPath), 0755)
			if err != nil {
				return err
			}

			err = isoExtractAll(dstPath, fullPath, fs)
			if err != nil {
				return err
			}
			continue
		}

		// Create the file
		f, err := fs.OpenFile(fullPath, os.O_RDONLY)
		if err != nil {
			return err
		}

		isoFile := f.(*iso9660.File)

		dest, err := os.Create(filepath.Join(dstPath, fullPath))
		if err != nil {
			return err
		}
		defer dest.Close()

		_, err = io.Copy(dest, isoFile)
		if err != nil {
			return err
		}
	}
	return nil
}

func imgUnpackIgnition(tmpPath string) error {
	ignitionImgPath := filepath.Join(tmpPath, "iso", "images", "ignition.img")

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

		if hdr.Name != "config.ign" {
			return fmt.Errorf("invalid ignition file found: %s", hdr.Name)
		}

		dest, err := os.Create(filepath.Join(tmpPath, "source-config.ign"))
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

func addFileToIgnitionConfig(tmpPath string, addFile string) error {
	sourceIgnition := filepath.Join(tmpPath, "source-config.ign")
	destIgnition := filepath.Join(tmpPath, "config.ign")

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
			Path:      fmt.Sprintf("/usr/local/bin/%s", filepath.Base(addFile)),
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

func imgRepackIgnition(tmpPath string) error {
	configFilePath := filepath.Join(tmpPath, "config.ign")
	newIgnitionImgPath := filepath.Join(tmpPath, "ignition.img")

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

func makeNewBootableISO(tmpPath string, destIsoPath string) error {

	// Copy the updated ignition archive in the temp folder
	srcFile := filepath.Join(tmpPath, "ignition.img")
	dstFile := filepath.Join(tmpPath, "iso", "images", "ignition.img")

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

	// Make the new ISO
	err = createBootableIso(tmpPath, destIsoPath)
	if err != nil {
		return err
	}

	return nil
}

func createBootableIso(tmpPath string, destIsoPath string) error {

	log.Printf("Creating the new iso %s\n", destIsoPath)

	os.Remove(destIsoPath)

	d, err := diskfs.Create(destIsoPath, 8712192, diskfs.Raw)
	if err != nil {
		return err
	}

	d.LogicalBlocksize = 2048
	fspec := disk.FilesystemSpec{
		Partition:   0,
		FSType:      filesystem.TypeISO9660,
		VolumeLabel: "rhcos-412.86.202209302317-0",
	}
	fs, err := d.CreateFilesystem(fspec)
	if err != nil {
		return err
	}

	filesPath := filepath.Join(tmpPath, "iso")

	// Add all files to the ISO
	addFileToISO := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		p, err := filepath.Rel(filesPath, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return fs.Mkdir(p)
		}

		content, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		rw, err := fs.OpenFile(p, os.O_CREATE|os.O_RDWR)
		if err != nil {
			return err
		}

		_, err = rw.Write(content)
		return err
	}
	if err := filepath.Walk(filesPath, addFileToISO); err != nil {
		return err
	}

	iso, ok := fs.(*iso9660.FileSystem)
	if !ok {
		return fmt.Errorf("not an iso9660 filesystem")
	}

	options := iso9660.FinalizeOptions{
		VolumeIdentifier: "rhcos-412.86.202209302317-0",
		ElTorito: &iso9660.ElTorito{
			BootCatalog: "isolinux/boot.cat",
			Entries: []*iso9660.ElToritoEntry{
				{
					Platform:  iso9660.BIOS,
					Emulation: iso9660.NoEmulation,
					BootFile:  "isolinux/isolinux.bin",
					BootTable: true,
					LoadSize:  4,
				},
				{
					Platform:  iso9660.EFI,
					Emulation: iso9660.NoEmulation,
					BootFile:  "images/efiboot.img",
				},
			},
		},
	}

	return iso.Finalize(options)
}
