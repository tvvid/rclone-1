package mockfs

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/hash"
)

// Fs is a minimal mock Fs
type Fs struct {
	name     string       // the name of the remote
	root     string       // The root directory (OS path)
	features *fs.Features // optional features
}

// ErrNotImplemented is returned by unimplemented methods
var ErrNotImplemented = errors.New("not implemented")

// NewFs returns a new mock Fs
func NewFs(name, root string) *Fs {
	f := &Fs{
		name: name,
		root: root,
	}
	f.features = (&fs.Features{}).Fill(f)
	return f
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String returns a description of the FS
func (f *Fs) String() string {
	return fmt.Sprintf("Mock file system at %s", f.root)
}

// Precision of the ModTimes in this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Hashes returns the supported hash types of the filesystem
func (f *Fs) Hashes() hash.Set {
	return hash.NewHashSet()
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(dir string) (entries fs.DirEntries, err error) {
	return nil, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	return nil, fs.ErrorObjectNotFound
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, ErrNotImplemented
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(dir string) error {
	return ErrNotImplemented
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(dir string) error {
	return ErrNotImplemented
}

// Assert it is the correct type
var _ fs.Fs = (*Fs)(nil)
