package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/FranGM/simplelog"
)

// getBackupDirName generates a directory name to store backups
//   based on the current time.
func (dfo *dfoState) getBackupDirName() string {
	if dfo.backupDir != "" {
		return dfo.backupDir
	}

	t := time.Now()
	b, _ := t.MarshalText()

	curDate := string(b)
	dirName := fmt.Sprintf("backups/dfo_backup_%v", curDate)
	dfo.backupDir = filepath.Join(dfo.config.WorkDir, dirName)
	return dfo.backupDir
}

// createBackupDir creates a backup directory for the dotfiles.
// If directory already exists no errors will be reported
func (dfo *dfoState) createBackupDir(backupDir string) error {
	simplelog.Debug.Printf("Ensuring backup directory (%q) exists", backupDir)
	if dfo.config.Noop {
		return nil
	}
	err := os.Mkdir(backupDir, 0755)
	if os.IsExist(err) {
		return nil
	}
	return err
}

// fileNeedsUpdating returns true if the file should be updated. This means either:
//   - File doesn't exist
//   - File is not a symlink
//   - File is a symlink to a different file
// Returns: needsUpdate, err
func fileNeedsUpdating(path string, newSrc string, config dfoConfig) (bool, error) {
	targetPath := filepath.Join(config.HomeDir, path)
	// We don't really care if it's a symlink or not, we just want to know if it's the same symlink we're going to create
	fi, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}

	if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
		linkTarget, err := os.Readlink(targetPath)
		if err != nil {
			return false, err
		}
		absSrc := filepath.Join(config.RepoDir, newSrc)
		// TODO: There's probably a better way of comparing them
		if absSrc == linkTarget {
			return false, nil
		}
	}
	return true, nil
}

// backupFile takes a backup of the given file and stores it in the backup directory
// path of the file to be backed up is relative to the user's home dir
func (dfo *dfoState) backupFile(path string) error {
	simplelog.Info.Printf("Backing up %q", path)

	srcPath := filepath.Join(dfo.config.HomeDir, path)
	targetBackupPath := filepath.Join(dfo.getBackupDirName(), path)
	targetDir := filepath.Dir(targetBackupPath)

	// If there's no source file there's nothing to backup
	fi, err := os.Stat(srcPath)
	if os.IsNotExist(err) {
		return nil
	}

	// Create backup directory if it doesn't exist already
	err = dfo.createBackupDir(dfo.getBackupDirName())
	if err != nil {
		return err
	}

	simplelog.Debug.Printf("Ensuring %q exists before backing up file", targetDir)
	// Create any subdirectories we might need
	if !dfo.config.Noop {
		err = os.MkdirAll(targetDir, 0755)
		if err != nil {
			return err
		}
	}

	simplelog.Debug.Printf("Backing up %q into %q", srcPath, targetBackupPath)

	if dfo.config.Noop {
		return nil
	}

	if fi.IsDir() {
		return copyDir(srcPath, targetBackupPath)
	}

	return os.Link(srcPath, targetBackupPath)
}

// replaceFile replaces a existing file with a symlink to src
// target file should have been backed up previously
func (dfo *dfoState) replaceFile(target string, src string) error {
	targetPath := filepath.Join(dfo.config.HomeDir, target)

	if dfo.config.Backup {
		err := dfo.backupFile(target)
		if err != nil {
			return err
		}
	}

	targetDir := filepath.Dir(targetPath)

	simplelog.Debug.Printf("Deleting %q", targetPath)
	if !dfo.config.Noop {
		err := os.RemoveAll(targetPath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	simplelog.Debug.Printf("Making sure %q exists", targetDir)
	if !dfo.config.Noop {
		// Make sure that the directory holding our file exists
		err := os.MkdirAll(targetDir, 0755)
		if err != nil {
			return err
		}
	}

	// Paths in dfo.yaml can be either absolute or relative to our dotfiles repo
	absSrc := src
	if !filepath.IsAbs(src) {
		absSrc = filepath.Join(dfo.config.RepoDir, src)
	}

	simplelog.Info.Printf("%q -> %q", absSrc, targetPath)
	if dfo.config.Noop {
		return nil
	}
	err := os.Symlink(absSrc, targetPath)
	return err
}

func copyDir(srcPath string, destPath string) error {

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	// create dest dir
	err = os.MkdirAll(destPath, srcInfo.Mode())
	if err != nil {
		return err
	}

	dir, _ := os.Open(srcPath)
	objects, err := dir.Readdir(-1)

	for _, obj := range objects {
		srcFile := filepath.Join(srcPath, obj.Name())
		destFile := filepath.Join(destPath, obj.Name())

		if obj.IsDir() {
			err = copyDir(srcFile, destFile)
			if err != nil {
				return err
			}
		} else {
			err = os.Link(srcFile, destFile)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
