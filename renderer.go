package hcat

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const (
	// DefaultFilePerms are the default file permissions for files rendered onto
	// disk when a specific file permission has not already been specified.
	defaultFilePerms = 0644
)

var (
	// ErrNoParentDir is the error returned with the parent directory is missing
	// and the user disabled it.
	errNoParentDir = errors.New("parent directory is missing")

	// ErrMissingDest is the error returned with the destination is empty.
	errMissingDest = errors.New("missing destination")
)

// FileRenderer will handle rendering the template text to a file.
type FileRenderer struct {
	createDestDirs bool
	path           string
	perms          os.FileMode
	backup         BackupFunc
}

// check for innterface compliance
var _ Renderer = (*FileRenderer)(nil)

// NewFileRenderer returns a new FileRenderer.
func NewFileRenderer(i FileRendererInput) FileRenderer {
	backup := i.Backup
	if backup == nil {
		backup = func(string) {}
	}
	return FileRenderer{
		createDestDirs: i.CreateDestDirs,
		path:           i.Path,
		perms:          i.Perms,
		backup:         backup,
	}
}

// FileRendererInput is the input structure for NewFileRenderer.
type FileRendererInput struct {
	// CreateDestDirs causes missing directories on path to be created
	CreateDestDirs bool
	// Path is the full file path to write to
	Path string
	// Perms sets the mode of the file
	Perms os.FileMode
	// Backup causes a backup of the rendered file to be made
	Backup BackupFunc
}

// BackupFunc defines the function type passed in to make backups if previously
// rendered templates, if desired.
type BackupFunc func(path string)

// RenderResult is returned and stored. It contains the status of the render
// operation.
type RenderResult struct {
	// DidRender indicates if the template rendered to disk. This will be false
	// in the event of an error, but it will also be false in dry mode or when
	// the template on disk matches the new result.
	DidRender bool

	// WouldRender indicates if the template would have rendered to disk. This
	// will return false in the event of an error, but will return true in dry
	// mode or when the template on disk matches the new result.
	WouldRender bool
}

// Render atomically renders a file contents to disk, returning a result of
// whether it would have rendered and actually did render.
func (r FileRenderer) Render(contents []byte) (RenderResult, error) {
	existing, err := ioutil.ReadFile(r.path)
	fileExists := !os.IsNotExist(err)
	if err != nil && fileExists {
		return RenderResult{}, errors.Wrap(err, "failed reading file")
	}

	if bytes.Equal(existing, contents) && fileExists {
		return RenderResult{
			DidRender:   false,
			WouldRender: true,
		}, nil
	}

	r.backup(r.path)

	err = atomicWrite(r.path, contents, r.perms, r.createDestDirs)
	if err != nil {
		return RenderResult{}, errors.Wrap(err, "failed writing file")
	}

	return RenderResult{
		DidRender:   true,
		WouldRender: true,
	}, nil
}

// Backup creates a [filename].bak copy, preserving the Mode
// Provided for convenience (to use as the BackupFunc) and an example.
func Backup(path string) {
	if path == "" {
		return
	}
	bak, old := path+".bak", path+".old.bak"
	os.Rename(bak, old) // ignore error
	if err := os.Link(path, bak); err == nil {
		os.Remove(old) // ignore error
	}
}

// AtomicWrite accepts a destination path and the template contents. It writes
// the template contents to a TempFile on disk, returning if any errors occur.
//
// If the parent destination directory does not exist, it will be created
// automatically with permissions 0755. To use a different permission, create
// the directory first or use `chmod` in a Command.
//
// If the destination path exists, all attempts will be made to preserve the
// existing file permissions. If those permissions cannot be read, an error is
// returned. If the file does not exist, it will be created automatically with
// permissions 0644. To use a different permission, create the destination file
// first or use `chmod` in a Command.
//
// If no errors occur, the Tempfile is "renamed" (moved) to the destination
// path.
func atomicWrite(
	path string, contents []byte, perms os.FileMode, createDestDirs bool,
) error {
	if path == "" {
		return errMissingDest
	}

	parent := filepath.Dir(path)
	if _, err := os.Stat(parent); os.IsNotExist(err) {
		if createDestDirs {
			if err := os.MkdirAll(parent, 0755); err != nil {
				return err
			}
		} else {
			return errNoParentDir
		}
	}

	f, err := ioutil.TempFile(parent, "")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())

	if _, err := f.Write(contents); err != nil {
		return err
	}

	if err := f.Sync(); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	// If the user did not explicitly set permissions, attempt to lookup the
	// current permissions on the file. If the file does not exist, fall back to
	// the default. Otherwise, inherit the current permissions.
	if perms == 0 {
		currentInfo, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				perms = defaultFilePerms
			} else {
				return err
			}
		} else {
			perms = currentInfo.Mode()

			// The file exists, so try to preserve the ownership as well.
			preserveFilePermissions(f.Name(), currentInfo)
		}
	}

	if err := os.Chmod(f.Name(), perms); err != nil {
		return err
	}

	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}

	return nil
}
