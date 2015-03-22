package main

import (
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type dfoConfig struct {
	RepoDir   string // Directory where to store dotfiles repo
	HomeDir   string // User's home directory. Relative target paths will be relative to this
	WorkDir   string // dfo's work directory (~/.dfo)
	GitRepo   string // Git repository that stores user's dotfiles
	Noop      bool   // Do not replace any files
	Verbose   bool   // Run dfo in verbose mode
	Backup    bool   // Make backups of files before replacing them
	UpdateGit bool   // Update dotfiles repo from origin before applying any changes
}

func (c *dfoConfig) loadConfig() error {
	configLocation := filepath.Join(c.WorkDir, "config.yaml")
	configBytes, err := ioutil.ReadFile(configLocation)
	if err != nil {
		// Not required to have a config.yaml
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	err = yaml.Unmarshal(configBytes, c)
	return err
}

func (c *dfoConfig) setDefaults() {
	c.HomeDir = os.Getenv("HOME")
	c.Backup = true
	c.WorkDir = filepath.Join(c.HomeDir, ".dfo")
	c.RepoDir = filepath.Join(c.WorkDir, "dotfiles")
	c.UpdateGit = true
}

func (c *dfoConfig) initFromParams() {

	flag.StringVar(&c.WorkDir, "workdir", c.WorkDir, "Work directory for dfo (will be used to store backups and dotfiles git repo)")
	flag.StringVar(&c.GitRepo, "gitrepo", c.GitRepo, "Remote git repo that holds your dotfiles (in the same format git would take it)")
	flag.BoolVar(&c.Noop, "noop", c.Noop, "Do a dry-run (don't replace any files)")
	flag.BoolVar(&c.Verbose, "verbose", c.Verbose, "Verbose output")
	flag.BoolVar(&c.Backup, "backup", c.Backup, "Perform backups of files that are updated")
	flag.BoolVar(&c.UpdateGit, "updategit", c.UpdateGit, "Do a 'git pull' and update submodules of the git repo")

	flag.Parse()
}
