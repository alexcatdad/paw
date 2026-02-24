package fsx

import "os"

type FS interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	ReadDir(name string) ([]os.DirEntry, error)
	MkdirAll(path string, perm os.FileMode) error
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	Stat(name string) (os.FileInfo, error)
	Lstat(name string) (os.FileInfo, error)
	Readlink(name string) (string, error)
	Symlink(oldname string, newname string) error
	Remove(name string) error
	RemoveAll(path string) error
	Rename(oldpath string, newpath string) error
	Chmod(name string, mode os.FileMode) error
}

type OSFS struct{}

func NewOSFS() FS {
	return OSFS{}
}

func (OSFS) ReadFile(name string) ([]byte, error) { return os.ReadFile(name) }
func (OSFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}
func (OSFS) ReadDir(name string) ([]os.DirEntry, error)   { return os.ReadDir(name) }
func (OSFS) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func (OSFS) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}
func (OSFS) Stat(name string) (os.FileInfo, error)        { return os.Stat(name) }
func (OSFS) Lstat(name string) (os.FileInfo, error)       { return os.Lstat(name) }
func (OSFS) Readlink(name string) (string, error)         { return os.Readlink(name) }
func (OSFS) Symlink(oldname string, newname string) error { return os.Symlink(oldname, newname) }
func (OSFS) Remove(name string) error                     { return os.Remove(name) }
func (OSFS) RemoveAll(path string) error                  { return os.RemoveAll(path) }
func (OSFS) Rename(oldpath string, newpath string) error  { return os.Rename(oldpath, newpath) }
func (OSFS) Chmod(name string, mode os.FileMode) error    { return os.Chmod(name, mode) }
