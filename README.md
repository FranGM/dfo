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
When replacing any files in your file system dfo will make a backup in a timestamped directory in its working directory. For example, if it replaces .vimrc it might appear here:
    /home/fran/.dfo/backups/dfo_backup_2015-01-12T22:18:21.67341215Z/.vimrc
