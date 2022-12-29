package main

import (
	"es-backup/backup"
	"es-backup/config"
)

func main() {
	p := config.NewParams()
	s := backup.NewSnapshotter(p)
	s.Snapshot()
}
