package common

import "os"

type FileSystem interface {
	MkdirTemp(dir, pattern string) (string, error)
	CreateTemp(dir, pattern string) (*os.File, error)
	Open(name string) (*os.File, error)
	Create(name string) (*os.File, error)
	RemoveAll(path string) error
	Remove(name string) error
	Stat(name string) (os.FileInfo, error)
}

type OSFileSystem struct{}

func (OSFileSystem) MkdirTemp(dir, pattern string) (string, error) {
	return os.MkdirTemp(dir, pattern)
}

func (OSFileSystem) CreateTemp(dir, pattern string) (*os.File, error) {
	return os.CreateTemp(dir, pattern)
}

func (OSFileSystem) Open(name string) (*os.File, error) {
	return os.Open(name)
}

func (OSFileSystem) Create(name string) (*os.File, error) {
	return os.Create(name)
}

func (OSFileSystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (OSFileSystem) Remove(name string) error {
	return os.Remove(name)
}

func (OSFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}
