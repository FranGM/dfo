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
	Noop    bool
	Verbose bool
	Yaml    dotfilesDef
}

type dotfilesDef struct {
	Files map[string]string
}

var config dfoConfig

func init() {
	config.HomeDir = os.Getenv("HOME")
	flag.StringVar(&config.WorkDir, "workdir", filepath.Join(config.HomeDir, ".dfo"), "Work directory for dfo (will be used to store backups and dotfiles git repo)")
	flag.StringVar(&config.Repo, "gitrepo", "", "Remote git repo that holds your dotfiles")
	flag.BoolVar(&config.Noop, "noop", false, "Run in noop mode (just do a dry-run)")
	flag.BoolVar(&config.Verbose, "verbose", false, "Verbose output")

	flag.Parse()
	config.RepoDir = filepath.Join(config.WorkDir, "dotfiles")

	if err := initWorkDir(); err != nil {
		log.Fatal(err)
	}

	if config.Verbose {
		simplelog.SetThreshold(simplelog.LevelDebug)
	} else {
		simplelog.SetThreshold(simplelog.LevelInfo)
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

	simplelog.Debug.Printf("Fetching updates from remote git repo...")
	// Do a git pull
	cmd := exec.Command("git", "pull")
	cmd.Dir = config.RepoDir
	cmd.Stderr = &e
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s\n", err.Error(), e.String())
	}

	simplelog.Debug.Printf("Updating submodules...")
	// Initialize submodules
	cmd = exec.Command("git", "submodule", "update", "--init", "--recursive")
	cmd.Dir = config.RepoDir
	cmd.Stderr = &e
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s\n", err.Error(), e.String())
	}

	return nil
}

// getBackupDirName generates a directory name to store backups
//   based on the current time.
func getBackupDirName() string {
	t := time.Now()
	b, _ := t.MarshalText()

	curDate := string(b)
	dirName := fmt.Sprintf("backups/dfo_backup_%v", curDate)
	return filepath.Join(config.WorkDir, dirName)
}

// createBackupDir creates a backup directory for the dotfiles.
// If directory already exists no errors will be reported
func createBackupDir(backupDir string) error {
	simplelog.Debug.Printf("Ensuring backup directory (%q) exists", backupDir)
	if config.Noop {
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
func fileNeedsUpdating(path string, newSrc string) (bool, error) {
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
func backupFile(path string, backupDir string) error {
	simplelog.Info.Printf("Backing up %q", path)

	srcPath := filepath.Join(config.HomeDir, path)
	targetBackupPath := filepath.Join(backupDir, path)
	targetDir := filepath.Dir(targetBackupPath)

	// If there's no source file there's nothing to backup
	_, err := os.Stat(srcPath)
	if os.IsNotExist(err) {
		return nil
	}

	// Create backup directory if it doesn't exist already
	err = createBackupDir(backupDir)
	if err != nil {
		return err
	}

	simplelog.Debug.Printf("Ensuring %q exists before backing up file", targetDir)
	// Create any subdirectories we might need
	if !config.Noop {
		err = os.MkdirAll(targetDir, 0755)
		if err != nil {
			return err
		}
	}

	simplelog.Debug.Printf("Backing up %q into %q", srcPath, targetBackupPath)
	if config.Noop {
		return nil
	}
	err = os.Link(srcPath, targetBackupPath)
	return err
}

// replaceFile replaces a existing file with a symlink to src
// target file should have been backed up previously
func replaceFile(target string, src string, backupDir string) error {
	targetPath := filepath.Join(config.HomeDir, target)

	err := backupFile(targetPath, backupDir)
	if err != nil {
		return err
	}

	targetDir := filepath.Dir(targetPath)

	simplelog.Debug.Printf("Deleting %q", targetPath)
	if !config.Noop {
		err = os.Remove(targetPath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	simplelog.Debug.Printf("Making sure %q exists", targetDir)
	if !config.Noop {
		// Make sure that the directory holding our file exists
		err = os.MkdirAll(targetDir, 0755)
		if err != nil {
			return err
		}
	}

	// Paths in dfo.yaml can be either absolute or relative to our dotfiles repo
	absSrc := src
	if !filepath.IsAbs(src) {
		absSrc = filepath.Join(config.RepoDir, src)
	}

	simplelog.Info.Printf("%q -> %q", absSrc, targetPath)
	if config.Noop {
		return nil
	}
	err = os.Symlink(absSrc, targetPath)
	return err
}

func main() {
	backupDir := getBackupDirName()

	for target, src := range config.Yaml.Files {
		needsUpdate, err := fileNeedsUpdating(target, src)
		if err != nil {
			log.Fatal(err)
		}

		if !needsUpdate {
			simplelog.Debug.Printf("No changes needed for %v", target)
			continue
		}

		err = replaceFile(target, src, backupDir)
		if err != nil {
			simplelog.Fatal.Printf("Error replacing file: %q", err)
		}
	}
}
