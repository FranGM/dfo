// dfo: Quick script to generate symlinks to your dotfiles

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/FranGM/simplelog"
	"gopkg.in/yaml.v2"
)

type dfoConfig struct {
	RepoDir string
	HomeDir string
	WorkDir string
	Repo    string
	Execute bool
	Yaml    yamlConfig
}

type yamlConfig struct {
	Files map[string]string
}

var config dfoConfig

func initWorkDir() error {
	var perm os.FileMode = 0755
	if err := os.MkdirAll(config.WorkDir, perm); err != nil {
		return err
	}
	backupDir := filepath.Join(config.WorkDir, "backups")
	if err := os.MkdirAll(backupDir, perm); err != nil {
		return err
	}

	// We should have a clone of our git dotfiles repo here, create it if it doesn't exist
	_, err := os.Stat(config.RepoDir)

	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		// If no git repo has been defined and we don't have a working repo already we have a problem
		if config.Repo == "" {
			simplelog.Fatal.Printf("No git repo has been specified and no current working repo in %q, aborting", config.RepoDir)
		}

		simplelog.Info.Printf("Repo doesn't exist, cloning %q into %q...", config.Repo, config.RepoDir)
		if err := initializeGitRepo(); err != nil {
			return err
		}
		simplelog.Info.Printf("Repo cloned.")
	}

	// TODO: If a gitrepo has been specified, check that it's the same that we already have?

	simplelog.Info.Printf("Updating repo/submodules (might take a while if it's the first time)")
	if err := updateGitRepo(); err != nil {
		return err
	}
	simplelog.Info.Printf("...Done")

	return nil
}

// Clone our dotfiles git repo into our dfo working directory
func initializeGitRepo() error {
	cmd := exec.Command("git", "clone", config.Repo, config.RepoDir)

	var e bytes.Buffer
	cmd.Stderr = &e

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s\n", err.Error(), e.String())
	}
	return nil
}

// Does a git pull from the remote dotfiles git repo into our working copy
func updateGitRepo() error {

	var e bytes.Buffer
	cmd := exec.Command("git", "submodule", "update", "--init", "--recursive")
	cmd.Dir = config.RepoDir
	cmd.Stderr = &e
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s\n", err.Error(), e.String())
	}

	cmd = exec.Command("git", "pull")
	cmd.Dir = config.RepoDir
	cmd.Stderr = &e
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s\n", err.Error(), e.String())
	}

	return nil
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
	fi, err := os.Lstat(targetPath)
	if err != nil {
		// If it doesn't exist we need to update it but not back it up
		if os.IsNotExist(err) {
			return true, false, nil
		}
		return false, false, err
	}

	if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
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
func backupFile(path string, backupDir string) error {
	simplelog.Info.Printf("Backing up %q", path)
	targetPath := filepath.Join(config.HomeDir, path)

	targetBackupPath := filepath.Join(backupDir, path)

	targetDir := filepath.Dir(targetBackupPath)
	var perm os.FileMode = 0755
	err := os.MkdirAll(targetDir, perm)
	if err != nil {
		return err
	}

	err = os.Link(targetPath, targetBackupPath)
	return err
}

// replaceFile replaces a existing file with a symlink to src
// target file should have been backed up previously
func replaceFile(target string, src string) error {
	simplelog.Info.Printf("Replacing content of %q", target)
	targetPath := filepath.Join(config.HomeDir, target)

	err := os.Remove(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Make sure that the directory holding our file exists
	dir := filepath.Dir(targetPath)
	var perm os.FileMode = 0755
	err = os.MkdirAll(dir, perm)
	if err != nil {
		return err
	}

	var absSrc string
	if !filepath.IsAbs(src) {
		absSrc = filepath.Join(config.RepoDir, src)
	} else {
		absSrc = src
	}
	err = os.Symlink(absSrc, targetPath)
	return err
}

func init() {
	config.HomeDir = os.Getenv("HOME")
	flag.StringVar(&config.WorkDir, "workdir", filepath.Join(config.HomeDir, ".dfo"), "Work directory for dfo (will be used to store backups and git repo)")
	flag.StringVar(&config.Repo, "gitrepo", "", "Remote git repo that holds your dotfiles")
	flag.BoolVar(&config.Execute, "execute", false, "Apply the changes (otherwise it will just do a dry-run)")
	getUsage := flag.Bool("help", false, "Display this help message")

	flag.Parse()
	config.RepoDir = filepath.Join(config.WorkDir, "dotfiles")

	simplelog.SetThreshold(simplelog.LevelDebug)

	if *getUsage {
		flag.Usage()
		os.Exit(1)
	}

	if err := initWorkDir(); err != nil {
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

func main() {
	var backupDir string

	for target, src := range config.Yaml.Files {
		simplelog.Info.Printf("%v -> %v", src, target)

		needsUpdate, needsBackup, err := fileNeedsUpdating(target, src)
		if err != nil {
			log.Fatal(err)
		}
		if !needsUpdate {
			simplelog.Info.Printf("No changes needed for %v", target)
			continue
		}

		if needsBackup {
			if backupDir == "" {
				backupDir, err = createBackupDir()
				if err != nil {
					log.Fatal(err)
				}
			}

			if !config.Execute {
				simplelog.Info.Printf("Would be backing up %q now...", target)
			} else {
				err = backupFile(target, backupDir)
				if err != nil {
					log.Fatal(err)
				}
			}
		}

		if !config.Execute {
			simplelog.Info.Printf("Would be replacing %q now...", target)
		} else {
			err = replaceFile(target, src)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}
