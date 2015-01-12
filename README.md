dfo
===

Yet another incredibly boring dotfiles organizer.

Installing:
  go get github.com/FranGM/dfo

Usage:
  Usage of dfo:
    -execute=false: Apply the changes (otherwise it will just do a dry-run)
    -gitrepo="": Remote git repo that holds your dotfiles
    -help=false: Display this help message
    -workdir="/home/fran/.dfo": Work directory for dfo (will be used to store backups and git repo)

gitrepo only needs to be specified in the first run, when dfo will clone it. In subsequent runs, only -execute needs to be specified.
