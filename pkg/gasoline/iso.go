package gasoline

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/filesystem/iso9660"
)

const (
	isoTempSubFolder string = "iso"
)

// isoUnpack extracts all files found in the iso image
// into the 'iso' subfolder within the temp one
func IsoUnpack(tmpPath string, isoPath string) error {
	// Create tmp folder for unpacking the source iso
	tmpIsoPath := filepath.Join(tmpPath, isoTempSubFolder)
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

// Recursively extracts all files and create the required folders
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

// Creates a new bootable iso image using all the files found
// in the temporary iso folder
func CreateBootableIso(tmpPath string, destIsoPath string) error {

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

	filesPath := filepath.Join(tmpPath, isoTempSubFolder)

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
