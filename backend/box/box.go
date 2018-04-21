// Package box provides an interface to the Box
// object storage system.
package box

// FIXME Box only supports file names of 255 characters or less. Names
// that will not be supported are those that contain non-printable
// ascii, / or \, names with trailing spaces, and the special names
// “.” and “..”.

// FIXME box can copy a directory

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ncw/rclone/backend/box/api"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config"
	"github.com/ncw/rclone/fs/config/flags"
	"github.com/ncw/rclone/fs/config/obscure"
	"github.com/ncw/rclone/fs/fserrors"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/lib/dircache"
	"github.com/ncw/rclone/lib/oauthutil"
	"github.com/ncw/rclone/lib/pacer"
	"github.com/ncw/rclone/lib/rest"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

const (
	rcloneClientID              = "d0374ba6pgmaguie02ge15sv1mllndho"
	rcloneEncryptedClientSecret = "sYbJYm99WB8jzeaLPU0OPDMJKIkZvD2qOn3SyEMfiJr03RdtDt3xcZEIudRhbIDL"
	minSleep                    = 10 * time.Millisecond
	maxSleep                    = 2 * time.Second
	decayConstant               = 2   // bigger for slower decay, exponential
	rootID                      = "0" // ID of root folder is always this
	rootURL                     = "https://api.box.com/2.0"
	uploadURL                   = "https://upload.box.com/api/2.0"
	listChunks                  = 1000     // chunk size to read directory listings
	minUploadCutoff             = 50000000 // upload cutoff can be no lower than this
)

// Globals
var (
	// Description of how to auth for this app
	oauthConfig = &oauth2.Config{
		Scopes: nil,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://app.box.com/api/oauth2/authorize",
			TokenURL: "https://app.box.com/api/oauth2/token",
		},
		ClientID:     rcloneClientID,
		ClientSecret: obscure.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectURL,
	}
	uploadCutoff = fs.SizeSuffix(50 * 1024 * 1024)
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "box",
		Description: "Box",
		NewFs:       NewFs,
		Config: func(name string) {
			err := oauthutil.Config("box", name, oauthConfig)
			if err != nil {
				log.Fatalf("Failed to configure token: %v", err)
			}
		},
		Options: []fs.Option{{
			Name: config.ConfigClientID,
			Help: "Box App Client Id - leave blank normally.",
		}, {
			Name: config.ConfigClientSecret,
			Help: "Box App Client Secret - leave blank normally.",
		}},
	})
	flags.VarP(&uploadCutoff, "box-upload-cutoff", "", "Cutoff for switching to multipart upload")
}

// Fs represents a remote box
type Fs struct {
	name         string                // name of this remote
	root         string                // the path we are working on
	features     *fs.Features          // optional features
	srv          *rest.Client          // the connection to the one drive server
	dirCache     *dircache.DirCache    // Map of directory path to directory id
	pacer        *pacer.Pacer          // pacer for API calls
	tokenRenewer *oauthutil.Renew      // renew the token on expiry
	uploadToken  *pacer.TokenDispenser // control concurrency
}

// Object describes a box object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          string    // ID of the object
	sha1        string    // SHA-1 of the object content
}

// ------------------------------------------------------------

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("box root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Pattern to match a box path
var matcher = regexp.MustCompile(`^([^/]*)(.*)$`)

// parsePath parses an box 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	429, // Too Many Requests.
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
	509, // Bandwidth Limit Exceeded
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func shouldRetry(resp *http.Response, err error) (bool, error) {
	authRety := false

	if resp != nil && resp.StatusCode == 401 && len(resp.Header["Www-Authenticate"]) == 1 && strings.Index(resp.Header["Www-Authenticate"][0], "expired_token") >= 0 {
		authRety = true
		fs.Debugf(nil, "Should retry: %v", err)
	}
	return authRety || fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// substitute reserved characters for box
func replaceReservedChars(x string) string {
	// Backslash for FULLWIDTH REVERSE SOLIDUS
	return strings.Replace(x, "\\", "＼", -1)
}

// restore reserved characters for box
func restoreReservedChars(x string) string {
	// FULLWIDTH REVERSE SOLIDUS for Backslash
	return strings.Replace(x, "＼", "\\", -1)
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(path string) (info *api.Item, err error) {
	// defer fs.Trace(f, "path=%q", path)("info=%+v, err=%v", &info, &err)
	leaf, directoryID, err := f.dirCache.FindRootAndPath(path, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	found, err := f.listAll(directoryID, false, true, func(item *api.Item) bool {
		if item.Name == leaf {
			info = item
			return true
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fs.ErrorObjectNotFound
	}
	return info, nil
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	// Decode error response
	errResponse := new(api.Error)
	err := rest.DecodeJSON(resp, &errResponse)
	if err != nil {
		fs.Debugf(nil, "Couldn't decode error response: %v", err)
	}
	if errResponse.Code == "" {
		errResponse.Code = resp.Status
	}
	if errResponse.Status == 0 {
		errResponse.Status = resp.StatusCode
	}
	return errResponse
}

// NewFs constructs an Fs from the path, container:path
func NewFs(name, root string) (fs.Fs, error) {
	if uploadCutoff < minUploadCutoff {
		return nil, errors.Errorf("box: upload cutoff (%v) must be greater than equal to %v", uploadCutoff, fs.SizeSuffix(minUploadCutoff))
	}

	root = parsePath(root)
	oAuthClient, ts, err := oauthutil.NewClient(name, oauthConfig)
	if err != nil {
		log.Fatalf("Failed to configure Box: %v", err)
	}

	f := &Fs{
		name:        name,
		root:        root,
		srv:         rest.NewClient(oAuthClient).SetRoot(rootURL),
		pacer:       pacer.New().SetMinSleep(minSleep).SetMaxSleep(maxSleep).SetDecayConstant(decayConstant),
		uploadToken: pacer.NewTokenDispenser(fs.Config.Transfers),
	}
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		CanHaveEmptyDirectories: true,
	}).Fill(f)
	f.srv.SetErrorHandler(errorHandler)

	// Renew the token in the background
	f.tokenRenewer = oauthutil.NewRenew(f.String(), ts, func() error {
		_, err := f.readMetaDataForPath("")
		return err
	})

	// Get rootID
	f.dirCache = dircache.New(root, rootID, f)

	// Find the current root
	err = f.dirCache.FindRoot(false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		newF := *f
		newF.dirCache = dircache.New(newRoot, rootID, &newF)
		newF.root = newRoot
		// Make new Fs which is the parent
		err = newF.dirCache.FindRoot(false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		_, err := newF.newObjectWithInfo(remote, nil)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				// File doesn't exist so return old f
				return f, nil
			}
			return nil, err
		}
		// return an error with an fs which points to the parent
		return &newF, fs.ErrorIsFile
	}
	return f, nil
}

// rootSlash returns root with a slash on if it is empty, otherwise empty string
func (f *Fs) rootSlash() string {
	if f.root == "" {
		return f.root
	}
	return f.root + "/"
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(remote string, info *api.Item) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		// Set info
		err = o.setMetaData(info)
	} else {
		err = o.readMetaData() // reads info and meta, returning an error
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	return f.newObjectWithInfo(remote, nil)
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(pathID, leaf string) (pathIDOut string, found bool, err error) {
	// Find the leaf in pathID
	found, err = f.listAll(pathID, true, false, func(item *api.Item) bool {
		if item.Name == leaf {
			pathIDOut = item.ID
			return true
		}
		return false
	})
	return pathIDOut, found, err
}

// fieldsValue creates a url.Values with fields set to those in api.Item
func fieldsValue() url.Values {
	values := url.Values{}
	values.Set("fields", api.ItemFields)
	return values
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(pathID, leaf string) (newID string, err error) {
	// fs.Debugf(f, "CreateDir(%q, %q)\n", pathID, leaf)
	var resp *http.Response
	var info *api.Item
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/folders",
		Parameters: fieldsValue(),
	}
	mkdir := api.CreateFolder{
		Name: replaceReservedChars(leaf),
		Parent: api.Parent{
			ID: pathID,
		},
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(&opts, &mkdir, &info)
		return shouldRetry(resp, err)
	})
	if err != nil {
		//fmt.Printf("...Error %v\n", err)
		return "", err
	}
	// fmt.Printf("...Id %q\n", *info.Id)
	return info.ID, nil
}

// list the objects into the function supplied
//
// If directories is set it only sends directories
// User function to process a File item from listAll
//
// Should return true to finish processing
type listAllFn func(*api.Item) bool

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) listAll(dirID string, directoriesOnly bool, filesOnly bool, fn listAllFn) (found bool, err error) {
	opts := rest.Opts{
		Method:     "GET",
		Path:       "/folders/" + dirID + "/items",
		Parameters: fieldsValue(),
	}
	opts.Parameters.Set("limit", strconv.Itoa(listChunks))
	offset := 0
OUTER:
	for {
		opts.Parameters.Set("offset", strconv.Itoa(offset))

		var result api.FolderItems
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(&opts, nil, &result)
			return shouldRetry(resp, err)
		})
		if err != nil {
			return found, errors.Wrap(err, "couldn't list files")
		}
		for i := range result.Entries {
			item := &result.Entries[i]
			if item.Type == api.ItemTypeFolder {
				if filesOnly {
					continue
				}
			} else if item.Type == api.ItemTypeFile {
				if directoriesOnly {
					continue
				}
			} else {
				fs.Debugf(f, "Ignoring %q - unknown type %q", item.Name, item.Type)
				continue
			}
			if item.ItemStatus != api.ItemStatusActive {
				continue
			}
			item.Name = restoreReservedChars(item.Name)
			if fn(item) {
				found = true
				break OUTER
			}
		}
		offset += result.Limit
		if offset >= result.TotalCount {
			break
		}
	}
	return
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
	err = f.dirCache.FindRoot(false)
	if err != nil {
		return nil, err
	}
	directoryID, err := f.dirCache.FindDir(dir, false)
	if err != nil {
		return nil, err
	}
	var iErr error
	_, err = f.listAll(directoryID, false, false, func(info *api.Item) bool {
		remote := path.Join(dir, info.Name)
		if info.Type == api.ItemTypeFolder {
			// cache the directory ID for later lookups
			f.dirCache.Put(remote, info.ID)
			d := fs.NewDir(remote, info.ModTime()).SetID(info.ID)
			// FIXME more info from dir?
			entries = append(entries, d)
		} else if info.Type == api.ItemTypeFile {
			o, err := f.newObjectWithInfo(remote, info)
			if err != nil {
				iErr = err
				return true
			}
			entries = append(entries, o)
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	if iErr != nil {
		return nil, iErr
	}
	return entries, nil
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Returns the object, leaf, directoryID and error
//
// Used to create new objects
func (f *Fs) createObject(remote string, modTime time.Time, size int64) (o *Object, leaf string, directoryID string, err error) {
	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err = f.dirCache.FindRootAndPath(remote, true)
	if err != nil {
		return
	}
	// Temporary Object under construction
	o = &Object{
		fs:     f,
		remote: remote,
	}
	return o, leaf, directoryID, nil
}

// Put the object
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	exisitingObj, err := f.newObjectWithInfo(src.Remote(), nil)
	switch err {
	case nil:
		return exisitingObj, exisitingObj.Update(in, src, options...)
	case fs.ErrorObjectNotFound:
		// Not found so create it
		return f.PutUnchecked(in, src)
	default:
		return nil, err
	}
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(in, src, options...)
}

// PutUnchecked the object into the container
//
// This will produce an error if the object already exists
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) PutUnchecked(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	size := src.Size()
	modTime := src.ModTime()

	o, _, _, err := f.createObject(remote, modTime, size)
	if err != nil {
		return nil, err
	}
	return o, o.Update(in, src, options...)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(dir string) error {
	err := f.dirCache.FindRoot(true)
	if err != nil {
		return err
	}
	if dir != "" {
		_, err = f.dirCache.FindDir(dir, true)
	}
	return err
}

// deleteObject removes an object by ID
func (f *Fs) deleteObject(id string) error {
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       "/files/" + id,
		NoResponse: true,
	}
	return f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.Call(&opts)
		return shouldRetry(resp, err)
	})
}

// purgeCheck removes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(dir string, check bool) error {
	root := path.Join(f.root, dir)
	if root == "" {
		return errors.New("can't purge root directory")
	}
	dc := f.dirCache
	err := dc.FindRoot(false)
	if err != nil {
		return err
	}
	rootID, err := dc.FindDir(dir, false)
	if err != nil {
		return err
	}

	opts := rest.Opts{
		Method:     "DELETE",
		Path:       "/folders/" + rootID,
		Parameters: url.Values{},
		NoResponse: true,
	}
	opts.Parameters.Set("recursive", strconv.FormatBool(!check))
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(&opts)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "rmdir failed")
	}
	f.dirCache.FlushDir(dir)
	if err != nil {
		return err
	}
	return nil
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(dir string) error {
	return f.purgeCheck(dir, true)
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Copy src to this remote using server side copy operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	err := srcObj.readMetaData()
	if err != nil {
		return nil, err
	}

	srcPath := srcObj.fs.rootSlash() + srcObj.remote
	dstPath := f.rootSlash() + remote
	if strings.ToLower(srcPath) == strings.ToLower(dstPath) {
		return nil, errors.Errorf("can't copy %q -> %q as are same name when lowercase", srcPath, dstPath)
	}

	// Create temporary object
	dstObj, leaf, directoryID, err := f.createObject(remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	// Copy the object
	opts := rest.Opts{
		Method:     "POST",
		Path:       "/files/" + srcObj.id + "/copy",
		Parameters: fieldsValue(),
	}
	replacedLeaf := replaceReservedChars(leaf)
	copy := api.CopyFile{
		Name: replacedLeaf,
		Parent: api.Parent{
			ID: directoryID,
		},
	}
	var resp *http.Response
	var info *api.Item
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(&opts, &copy, &info)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, err
	}
	err = dstObj.setMetaData(info)
	if err != nil {
		return nil, err
	}
	return dstObj, nil
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge() error {
	return f.purgeCheck("", false)
}

// move a file or folder
func (f *Fs) move(endpoint, id, leaf, directoryID string) (info *api.Item, err error) {
	// Move the object
	opts := rest.Opts{
		Method:     "PUT",
		Path:       endpoint + id,
		Parameters: fieldsValue(),
	}
	move := api.UpdateFileMove{
		Name: replaceReservedChars(leaf),
		Parent: api.Parent{
			ID: directoryID,
		},
	}
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(&opts, &move, &info)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, err
	}
	return info, nil
}

// Move src to this remote using server side move operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	// Create temporary object
	dstObj, leaf, directoryID, err := f.createObject(remote, srcObj.modTime, srcObj.size)
	if err != nil {
		return nil, err
	}

	// Do the move
	info, err := f.move("/files/", srcObj.id, leaf, directoryID)
	if err != nil {
		return nil, err
	}

	err = dstObj.setMetaData(info)
	if err != nil {
		return nil, err
	}
	return dstObj, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	srcPath := path.Join(srcFs.root, srcRemote)
	dstPath := path.Join(f.root, dstRemote)

	// Refuse to move to or from the root
	if srcPath == "" || dstPath == "" {
		fs.Debugf(src, "DirMove error: Can't move root")
		return errors.New("can't move root directory")
	}

	// find the root src directory
	err := srcFs.dirCache.FindRoot(false)
	if err != nil {
		return err
	}

	// find the root dst directory
	if dstRemote != "" {
		err = f.dirCache.FindRoot(true)
		if err != nil {
			return err
		}
	} else {
		if f.dirCache.FoundRoot() {
			return fs.ErrorDirExists
		}
	}

	// Find ID of dst parent, creating subdirs if necessary
	var leaf, directoryID string
	findPath := dstRemote
	if dstRemote == "" {
		findPath = f.root
	}
	leaf, directoryID, err = f.dirCache.FindPath(findPath, true)
	if err != nil {
		return err
	}

	// Check destination does not exist
	if dstRemote != "" {
		_, err = f.dirCache.FindDir(dstRemote, false)
		if err == fs.ErrorDirNotFound {
			// OK
		} else if err != nil {
			return err
		} else {
			return fs.ErrorDirExists
		}
	}

	// Find ID of src
	srcID, err := srcFs.dirCache.FindDir(srcRemote, false)
	if err != nil {
		return err
	}

	// Do the move
	_, err = f.move("/folders/", srcID, leaf, directoryID)
	if err != nil {
		return err
	}
	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.SHA1)
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// srvPath returns a path for use in server
func (o *Object) srvPath() string {
	return replaceReservedChars(o.fs.rootSlash() + o.remote)
}

// Hash returns the SHA-1 of an object returning a lowercase hex string
func (o *Object) Hash(t hash.Type) (string, error) {
	if t != hash.SHA1 {
		return "", hash.ErrUnsupported
	}
	return o.sha1, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	err := o.readMetaData()
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return 0
	}
	return o.size
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.Item) (err error) {
	if info.Type != api.ItemTypeFile {
		return errors.Wrapf(fs.ErrorNotAFile, "%q is %q", o.remote, info.Type)
	}
	o.hasMetaData = true
	o.size = int64(info.Size)
	o.sha1 = info.SHA1
	o.modTime = info.ModTime()
	o.id = info.ID
	return nil
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData() (err error) {
	if o.hasMetaData {
		return nil
	}
	info, err := o.fs.readMetaDataForPath(o.remote)
	if err != nil {
		if apiErr, ok := err.(*api.Error); ok {
			if apiErr.Code == "not_found" || apiErr.Code == "trashed" {
				return fs.ErrorObjectNotFound
			}
		}
		return err
	}
	return o.setMetaData(info)
}

// ModTime returns the modification time of the object
//
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime() time.Time {
	err := o.readMetaData()
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	return o.modTime
}

// setModTime sets the modification time of the local fs object
func (o *Object) setModTime(modTime time.Time) (*api.Item, error) {
	opts := rest.Opts{
		Method:     "PUT",
		Path:       "/files/" + o.id,
		Parameters: fieldsValue(),
	}
	update := api.UpdateFileModTime{
		ContentModifiedAt: api.Time(modTime),
	}
	var info *api.Item
	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(&opts, &update, &info)
		return shouldRetry(resp, err)
	})
	return info, err
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(modTime time.Time) error {
	info, err := o.setModTime(modTime)
	if err != nil {
		return err
	}
	return o.setMetaData(info)
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(options ...fs.OpenOption) (in io.ReadCloser, err error) {
	if o.id == "" {
		return nil, errors.New("can't download - no id")
	}
	fs.FixRangeOption(options, o.size)
	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		Path:    "/files/" + o.id + "/content",
		Options: options,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(&opts)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

// upload does a single non-multipart upload
//
// This is recommended for less than 50 MB of content
func (o *Object) upload(in io.Reader, leaf, directoryID string, modTime time.Time) (err error) {
	upload := api.UploadFile{
		Name:              replaceReservedChars(leaf),
		ContentModifiedAt: api.Time(modTime),
		ContentCreatedAt:  api.Time(modTime),
		Parent: api.Parent{
			ID: directoryID,
		},
	}

	var resp *http.Response
	var result api.FolderItems
	opts := rest.Opts{
		Method: "POST",
		Body:   in,
		MultipartMetadataName: "attributes",
		MultipartContentName:  "contents",
		MultipartFileName:     upload.Name,
		RootURL:               uploadURL,
	}
	// If object has an ID then it is existing so create a new version
	if o.id != "" {
		opts.Path = "/files/" + o.id + "/content"
	} else {
		opts.Path = "/files/content"
	}
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(&opts, &upload, &result)
		return shouldRetry(resp, err)
	})
	if err != nil {
		return err
	}
	if result.TotalCount != 1 || len(result.Entries) != 1 {
		return errors.Errorf("failed to upload %v - not sure why", o)
	}
	return o.setMetaData(&result.Entries[0])
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one
//
// The new object may have been created if an error is returned
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	o.fs.tokenRenewer.Start()
	defer o.fs.tokenRenewer.Stop()

	size := src.Size()
	modTime := src.ModTime()
	remote := o.Remote()

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := o.fs.dirCache.FindRootAndPath(remote, true)
	if err != nil {
		return err
	}

	// Upload with simple or multipart
	if size <= int64(uploadCutoff) {
		err = o.upload(in, leaf, directoryID, modTime)
	} else {
		err = o.uploadMultipart(in, leaf, directoryID, size, modTime)
	}
	return err
}

// Remove an object
func (o *Object) Remove() error {
	return o.fs.deleteObject(o.id)
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.PutStreamer     = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
)
