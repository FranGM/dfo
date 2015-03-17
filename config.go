package main

import (
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type dfoConfig struct {
	RepoDir   string
	HomeDir   string
	WorkDir   string
	Repo      string
	Noop      bool
	Verbose   bool
	Backup    bool
	UpdateGit bool
}

func (c *dfoConfig) loadConfig() error {
	homeDir := os.Getenv("HOME")

	configLocation := filepath.Join(homeDir, ".dfo/config.yaml")
	configBytes, err := ioutil.ReadFile(configLocation)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil
		}
		return err
	}

	err = yaml.Unmarshal(configBytes, c)
	return err
}

func (c *dfoConfig) setDefaults() {
	c.HomeDir = os.Getenv("HOME")
	c.WorkDir = filepath.Join(c.HomeDir, ".dfo")
	c.RepoDir = filepath.Join(c.WorkDir, "dotfiles")
	c.UpdateGit = true
}

func (c *dfoConfig) initFromParams() {

	flag.StringVar(&c.WorkDir, "workdir", c.WorkDir, "Work directory for dfo (will be used to store backups and dotfiles git repo)")
	flag.StringVar(&c.Repo, "gitrepo", c.Repo, "Remote git repo that holds your dotfiles (in the same format git would take it)")
	flag.BoolVar(&c.Noop, "noop", c.Noop, "Run in noop mode (just do a dry-run)")
	flag.BoolVar(&c.Verbose, "verbose", c.Verbose, "Verbose output")
	flag.BoolVar(&c.Backup, "backup", c.Backup, "Perform backups of files that are updated")
	flag.BoolVar(&c.UpdateGit, "updategit", c.UpdateGit, "Do a 'git pull' and update submodules of the git repo")

	flag.Parse()
}
