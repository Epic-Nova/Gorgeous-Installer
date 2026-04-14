package bundle

import (
	"embed"
	"io/fs"
)

// Files contains installer assets embedded from project root at build time.
//
//go:embed config.json icon.png packs/**
var Files embed.FS

func ReadFile(name string) ([]byte, error) {
	return Files.ReadFile(name)
}

func ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(Files, name)
}

func Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(Files, name)
}
