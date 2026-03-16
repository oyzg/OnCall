package db

type Migrator struct {
	Path string
}

func NewMigrator(path string) Migrator {
	return Migrator{Path: path}
}
