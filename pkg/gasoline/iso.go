package gasoline

import (
	"fmt"
	"io"
	"log"
	"math"
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

	disk, err := diskfs.Open(isoPath, diskfs.WithOpenMode(diskfs.ReadOnly))
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

func fileExists(name string) (bool, error) {
	if _, err := os.Stat(name); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func efiLoadSectors(workDir string) (uint16, error) {
	efiStat, err := os.Stat(filepath.Join(workDir, "images/efiboot.img"))
	if err != nil {
		return 0, err
	}
	return uint16(math.Ceil(float64(efiStat.Size()) / 2048)), nil
}

func haveBootFiles(workDir string) (bool, error) {
	files := []string{"isolinux/boot.cat", "isolinux/isolinux.bin", "images/efiboot.img"}
	for _, f := range files {
		name := filepath.Join(workDir, f)
		if exists, err := fileExists(name); err != nil {
			return false, err
		} else if !exists {
			return false, nil
		}
	}

	return true, nil
}

// Creates a new bootable iso image using all the files found
// in the temporary iso folder
func CreateBootableIso(tmpPath string, destIsoPath string) error {

	log.Printf("Creating the new iso %s\n", destIsoPath)

	volumeLabel := "rhcos-413.86.202212131234-0"
	workDir := filepath.Join(tmpPath, isoTempSubFolder)

	os.Remove(destIsoPath)

	folderSize, err := FolderSize(workDir)
	if err != nil {
		return err
	}

	d, err := diskfs.Create(destIsoPath, folderSize, diskfs.Raw, diskfs.SectorSizeDefault)
	if err != nil {
		return err
	}

	d.LogicalBlocksize = 2048
	fspec := disk.FilesystemSpec{
		Partition:   0,
		FSType:      filesystem.TypeISO9660,
		VolumeLabel: volumeLabel,
		WorkDir:     workDir,
	}
	fs, err := d.CreateFilesystem(fspec)
	if err != nil {
		return err
	}

	iso, ok := fs.(*iso9660.FileSystem)
	if !ok {
		return fmt.Errorf("not an iso9660 filesystem")
	}

	options := iso9660.FinalizeOptions{
		RockRidge:        true,
		VolumeIdentifier: volumeLabel,
	}

	if haveFiles, err := haveBootFiles(workDir); err != nil {
		return err
	} else if haveFiles {
		efiSectors, err := efiLoadSectors(workDir)
		if err != nil {
			return err
		}
		options.ElTorito = &iso9660.ElTorito{
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
					LoadSize:  efiSectors,
				},
			},
		}
	} else if exists, _ := fileExists(filepath.Join(workDir, "images/efiboot.img")); exists {
		// Creating an ISO with EFI boot only
		efiSectors, err := efiLoadSectors(workDir)
		if err != nil {
			return err
		}
		if exists, _ := fileExists(filepath.Join(workDir, "boot.catalog")); !exists {
			return fmt.Errorf("missing boot.catalog file")
		}
		options.ElTorito = &iso9660.ElTorito{
			BootCatalog:     "boot.catalog",
			HideBootCatalog: true,
			Entries: []*iso9660.ElToritoEntry{
				{
					Platform:  iso9660.EFI,
					Emulation: iso9660.NoEmulation,
					BootFile:  "images/efiboot.img",
					LoadSize:  efiSectors,
				},
			},
		}
	}

	return iso.Finalize(options)
}

func FolderSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}
