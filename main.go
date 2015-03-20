// dfo: Quick script to generate symlinks to your dotfiles

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/FranGM/simplelog"
	"gopkg.in/yaml.v2"
)

type dotfileDef struct {
	src string
	dst string
}

type dfoState struct {
	config    dfoConfig
	backupDir string
	dotfiles  []dotfileDef
}

func (dfo *dfoState) initWorkDir() error {
	if err := os.MkdirAll(dfo.config.WorkDir, 0755); err != nil {
		return err
	}
	backupDir := filepath.Join(dfo.config.WorkDir, "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return err
	}

	// We should have a clone of our git dotfiles repo here, create it if it doesn't exist
	_, err := os.Stat(dfo.config.RepoDir)

	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		// If no git repo has been defined and we don't have a working repo already we have a problem
		if dfo.config.GitRepo == "" {
			simplelog.Fatal.Printf("No git repo has been specified and no current working repo in %q, aborting", dfo.config.RepoDir)
		}

		simplelog.Info.Printf("Repo doesn't exist, cloning %q into %q...", dfo.config.GitRepo, dfo.config.RepoDir)
		if err := initGitRepo(dfo.config); err != nil {
			return err
		}
		simplelog.Info.Printf("Repo cloned.")
	}

	// TODO: If a gitrepo has been specified, check that it's the same that we already have?

	if dfo.config.UpdateGit {
		simplelog.Info.Printf("Updating repo/submodules (might take a while if it's the first time)")
		if err := updateGitRepo(dfo.config); err != nil {
			return err
		}
		simplelog.Info.Printf("...Done")
	}

	return nil
}

// Clone our dotfiles git repo into our dfo working directory
func initGitRepo(c dfoConfig) error {
	cmd := exec.Command("git", "clone", c.GitRepo, c.RepoDir)

	var e bytes.Buffer
	cmd.Stderr = &e

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s\n", err.Error(), e.String())
	}

	return updateGitSubmodules(c)
}

// Does a git pull from the remote dotfiles git repo into our working copy
func updateGitRepo(c dfoConfig) error {
	var e bytes.Buffer

	simplelog.Debug.Printf("Fetching updates from remote git repo...")
	// Do a git pull
	cmd := exec.Command("git", "pull")
	cmd.Dir = c.RepoDir
	cmd.Stderr = &e
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s\n", err.Error(), e.String())
	}

	return updateGitSubmodules(c)
}

func updateGitSubmodules(c dfoConfig) error {
	var e bytes.Buffer

	simplelog.Debug.Printf("Updating git submodules...")
	cmd := exec.Command("git", "submodule", "update", "--init", "--recursive")
	cmd.Dir = c.RepoDir
	cmd.Stderr = &e
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s\n", err.Error(), e.String())
	}

	return nil
}

func main() {
	var dfo dfoState

	dfo.config.setDefaults()

	err := dfo.config.loadConfig()
	if err != nil {
		simplelog.Fatal.Printf("Error loading config file: %q", err)
	}

	dfo.config.initFromParams()

	if err := dfo.initWorkDir(); err != nil {
		log.Fatal(err)
	}

	if dfo.config.Verbose {
		simplelog.SetThreshold(simplelog.LevelDebug)
	} else {
		simplelog.SetThreshold(simplelog.LevelInfo)
	}

	configPath := filepath.Join(dfo.config.RepoDir, "dfo.yaml")
	configBytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatal(err)
	}

	m := make(map[string]string)
	err = yaml.Unmarshal(configBytes, &m)
	if err != nil {
		log.Fatal(err)
	}

	for dst, src := range m {
		dfo.dotfiles = append(dfo.dotfiles, dotfileDef{dst: dst, src: src})
	}

	simplelog.Debug.Printf("Backups will be stored in %q", dfo.getBackupDirName())

	for _, file := range dfo.dotfiles {
		needsUpdate, err := fileNeedsUpdating(file.dst, file.src, dfo.config)
		if err != nil {
			log.Fatal(err)
		}

		if !needsUpdate {
			simplelog.Debug.Printf("No changes needed for %v", file.dst)
			continue
		}

		err = dfo.replaceFile(file.dst, file.src)
		if err != nil {
			simplelog.Fatal.Println(err)
		}
	}
}
