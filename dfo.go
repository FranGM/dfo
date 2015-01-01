// dfo: Quick script to generate symlinks to your dotfiles

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v2"
)

type dfoConfig struct {
	RepoDir string
	HomeDir string
	WorkDir string
	Yaml    yamlConfig
}

type yamlConfig struct {
	Files map[string]string
}

var config dfoConfig

func initWorkDir() error {
	var perm os.FileMode = 0755
	err := os.MkdirAll(config.WorkDir, perm)
	if err != nil {
		return err
	}
	backupDir := filepath.Join(config.WorkDir, "backups")
	err = os.MkdirAll(backupDir, perm)
	return err
}

func populateConfigDefaults() {
	config.HomeDir = os.Getenv("HOME")
	config.RepoDir = filepath.Join(config.HomeDir, "git/dotfiles")
	config.WorkDir = filepath.Join(config.HomeDir, ".dfo")
}

// Env variables:
// DFO_REPODIR: Path to the dotfiles repo. Default: ~/git/dotfiles/
// DFO_WORKDIR: Path to the dfo work directory. Default: ~/.dfo/
func init() {

	populateConfigDefaults()

	err := envconfig.Process("dfo", &config)
	if err != nil {
		log.Fatal(err)
	}

	err = initWorkDir()
	if err != nil {
		log.Fatal(err)
	}

	configPath := filepath.Join(config.RepoDir, "dfo.yaml")
	configBytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatal(err)
	}
	err = yaml.Unmarshal(configBytes, &config.Yaml)
	if err != nil {
		log.Fatal(err)
	}
}

// TODO: Only create the backup dir when we're actually backing up files
// createBackupDir creates a backup directory for the dotfiles.
// Returns the name of the directory and any errors that appeared while creating it
func createBackupDir() (string, error) {
	t := time.Now()
	b, err := t.MarshalText()
	if err != nil {
		return "", err
	}
	curDate := string(b)
	dirName := fmt.Sprintf("backups/dfo_backup_%v", curDate)
	backupDir := filepath.Join(config.WorkDir, dirName)

	var perm os.FileMode = 0755
	err = os.Mkdir(backupDir, perm)
	return backupDir, err
}

// TODO: Change name
// TODO: Return a struct so we can say if it's a symlink, where it's pointing to, etc
// fileNeedsUpdating returns true if the file should be updated. This means either:
//   - File doesn't exist
//   - File is not a symlink
//   - File is a symlink to a different file
// Returns: needsUpdate, needsBackup, err
func fileNeedsUpdating(path string, newSrc string) (bool, bool, error) {

	targetPath := filepath.Join(config.HomeDir, path)
	// We don't really care if it's a symlink or not, we just want to know if it's the same symlink we're going to create
	// TODO: If the file doesn't exist, don't treat it as an error
	fi, err := os.Lstat(targetPath)
	if err != nil {
		// If it doesn't exist we need to update it but not back it up
		if os.IsNotExist(err) {
			return true, false, nil
		}
		return false, false, err
	}

	if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
		// TODO: Here we would make a note of where the symlink is pointing to for backup purposes
		linkTarget, err := os.Readlink(targetPath)
		if err != nil {
			return false, false, err
		}
		absSrc := filepath.Join(config.RepoDir, newSrc)
		// TODO: There's probably a better way of comparing them
		if absSrc == linkTarget {
			return false, false, nil
		}
	} else {
		return true, true, nil
	}
	return true, true, nil
}

// backupFile takes a backup of the given file and stores it in the backup directory
// TODO: Also keep track of what files (both source and target) have been backed up so they're easier to restore
func backupFile(path string, backupDir string) error {
	targetPath := filepath.Join(config.HomeDir, path)

	targetBackupPath := filepath.Join(backupDir, path)
	err := os.Link(targetPath, targetBackupPath)
	return err
}

// replaceFile replaces a existing file with a symlink to src
// target file should have been backed up previously
func replaceFile(target string, src string) error {
	targetPath := filepath.Join(config.HomeDir, target)

	// TODO: Handle when target doesn't exist
	err := os.Remove(targetPath)

	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// TODO: Check if path is absolute first?
	absSrc := filepath.Join(config.RepoDir, src)
	err = os.Symlink(absSrc, targetPath)
	return err
}

func main() {
	var backupDir string

	for target, src := range config.Yaml.Files {
		log.Printf("%v -> %v", target, src)

		needsUpdate, needsBackup, err := fileNeedsUpdating(target, src)
		if err != nil {
			log.Fatal(err)
		}
		if !needsUpdate {
			log.Printf("No changes needed for %v", target)
			continue
		}

		if needsBackup {
			if len(backupDir) == 0 {
				backupDir, err = createBackupDir()
				if err != nil {
					log.Fatal(err)
				}
			}
			err = backupFile(target, backupDir)
			if err != nil {
				log.Fatal(err)
			}
		}

		err = replaceFile(target, src)
		if err != nil {
			log.Fatal(err)
		}
	}
}
