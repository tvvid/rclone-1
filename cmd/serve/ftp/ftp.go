// Package ftp implements an FTP server for rclone

//+build !plan9

package ftp

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"strconv"
	"sync"

	ftp "github.com/goftp/server"
	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/cmd/serve/ftp/ftpflags"
	"github.com/ncw/rclone/cmd/serve/ftp/ftpopt"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/accounting"
	"github.com/ncw/rclone/fs/log"
	"github.com/ncw/rclone/vfs"
	"github.com/ncw/rclone/vfs/vfsflags"
	"github.com/spf13/cobra"
)

func init() {
	ftpflags.AddFlags(Command.Flags())
	vfsflags.AddFlags(Command.Flags())
}

// Command definition for cobra
var Command = &cobra.Command{
	Use:   "ftp remote:path",
	Short: `Serve remote:path over FTP.`,
	Long: `
rclone serve ftp implements a basic ftp server to serve the
remote over FTP protocol. This can be viewed with a ftp client
or you can make a remote of type ftp to read and write it.
` + ftpopt.Help + vfs.Help,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			s, err := newServer(f, &ftpflags.Opt)
			if err != nil {
				return err
			}
			return s.serve()
		})
	},
}

// server contains everything to run the server
type server struct {
	f   fs.Fs
	srv *ftp.Server
}

// Make a new FTP to serve the remote
func newServer(f fs.Fs, opt *ftpopt.Options) (*server, error) {
	host, port, err := net.SplitHostPort(opt.ListenAddr)
	if err != nil {
		return nil, errors.New("Failed to parse host:port")
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return nil, errors.New("Failed to parse host:port")
	}

	ftpopt := &ftp.ServerOpts{
		Name:           "Rclone FTP Server",
		WelcomeMessage: "Welcome on Rclone FTP Server",
		Factory: &DriverFactory{
			vfs: vfs.New(f, &vfsflags.Opt),
		},
		Hostname:     host,
		Port:         portNum,
		PassivePorts: opt.PassivePorts,
		Auth: &Auth{
			BasicUser: opt.BasicUser,
			BasicPass: opt.BasicPass,
		},
		Logger: &Logger{},
		//TODO implement a maximum of https://godoc.org/github.com/goftp/server#ServerOpts
	}
	return &server{
		f:   f,
		srv: ftp.NewServer(ftpopt),
	}, nil
}

// serve runs the ftp server
func (s *server) serve() error {
	fs.Logf(s.f, "Serving FTP on %s", s.srv.Hostname+":"+strconv.Itoa(s.srv.Port))
	return s.srv.ListenAndServe()
}

// serve runs the ftp server
func (s *server) close() error {
	fs.Logf(s.f, "Stopping FTP on %s", s.srv.Hostname+":"+strconv.Itoa(s.srv.Port))
	return s.srv.Shutdown()
}

//Logger ftp logger output formatted message
type Logger struct{}

//Print log simple text message
func (l *Logger) Print(sessionID string, message interface{}) {
	fs.Infof(sessionID, "%s", message)
}

//Printf log formatted text message
func (l *Logger) Printf(sessionID string, format string, v ...interface{}) {
	fs.Infof(sessionID, format, v...)
}

//PrintCommand log formatted command execution
func (l *Logger) PrintCommand(sessionID string, command string, params string) {
	if command == "PASS" {
		fs.Infof(sessionID, "> PASS ****")
	} else {
		fs.Infof(sessionID, "> %s %s", command, params)
	}
}

//PrintResponse log responses
func (l *Logger) PrintResponse(sessionID string, code int, message string) {
	fs.Infof(sessionID, "< %d %s", code, message)
}

//Auth struct to handle ftp auth (temporary simple for POC)
type Auth struct {
	BasicUser string
	BasicPass string
}

//CheckPasswd handle auth based on configuration
func (a *Auth) CheckPasswd(user, pass string) (bool, error) {
	return a.BasicUser == user && (a.BasicPass == "" || a.BasicPass == pass), nil
}

//DriverFactory factory of ftp driver for each session
type DriverFactory struct {
	vfs *vfs.VFS
}

//NewDriver start a new session
func (f *DriverFactory) NewDriver() (ftp.Driver, error) {
	log.Trace("", "Init driver")("")
	return &Driver{
		vfs: f.vfs,
	}, nil
}

//Driver impletation of ftp server
type Driver struct {
	vfs  *vfs.VFS
	lock sync.Mutex
}

//Init a connection
func (d *Driver) Init(*ftp.Conn) {
	defer log.Trace("", "Init session")("")
}

//Stat get information on file or folder
func (d *Driver) Stat(path string) (fi ftp.FileInfo, err error) {
	defer log.Trace(path, "")("fi=%+v, err = %v", &fi, &err)
	n, err := d.vfs.Stat(path)
	if err != nil {
		return nil, err
	}
	return &FileInfo{n, n.Mode(), d.vfs.Opt.UID, d.vfs.Opt.GID}, err
}

//ChangeDir move current folder
func (d *Driver) ChangeDir(path string) (err error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	defer log.Trace(path, "")("err = %v", &err)
	n, err := d.vfs.Stat(path)
	if err != nil {
		return err
	}
	if !n.IsDir() {
		return errors.New("Not a directory")
	}
	return nil
}

//ListDir list content of a folder
func (d *Driver) ListDir(path string, callback func(ftp.FileInfo) error) (err error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	defer log.Trace(path, "")("err = %v", &err)
	node, err := d.vfs.Stat(path)
	if err == vfs.ENOENT {
		return errors.New("Directory not found")
	} else if err != nil {
		return err
	}
	if !node.IsDir() {
		return errors.New("Not a directory")
	}

	dir := node.(*vfs.Dir)
	dirEntries, err := dir.ReadDirAll()
	if err != nil {
		return err
	}

	// Account the transfer
	accounting.Stats.Transferring(path)
	defer accounting.Stats.DoneTransferring(path, true)

	for _, file := range dirEntries {
		err = callback(&FileInfo{file, file.Mode(), d.vfs.Opt.UID, d.vfs.Opt.GID})
		if err != nil {
			return err
		}
	}
	return nil
}

//DeleteDir delete a folder and his content
func (d *Driver) DeleteDir(path string) (err error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	defer log.Trace(path, "")("err = %v", &err)
	node, err := d.vfs.Stat(path)
	if err != nil {
		return err
	}
	if !node.IsDir() {
		return errors.New("Not a directory")
	}
	err = node.Remove()
	if err != nil {
		return err
	}
	return nil
}

//DeleteFile delete a file
func (d *Driver) DeleteFile(path string) (err error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	defer log.Trace(path, "")("err = %v", &err)
	node, err := d.vfs.Stat(path)
	if err != nil {
		return err
	}
	if !node.IsFile() {
		return errors.New("Not a file")
	}
	err = node.Remove()
	if err != nil {
		return err
	}
	return nil
}

//Rename rename a file or folder
func (d *Driver) Rename(oldName, newName string) (err error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	defer log.Trace(oldName, "newName=%q", newName)("err = %v", &err)
	return d.vfs.Rename(oldName, newName)
}

//MakeDir create a folder
func (d *Driver) MakeDir(path string) (err error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	defer log.Trace(path, "")("err = %v", &err)
	dir, leaf, err := d.vfs.StatParent(path)
	if err != nil {
		return err
	}
	_, err = dir.Mkdir(leaf)
	return err
}

//GetFile download a file
func (d *Driver) GetFile(path string, offset int64) (size int64, fr io.ReadCloser, err error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	defer log.Trace(path, "offset=%v", offset)("err = %v", &err)
	node, err := d.vfs.Stat(path)
	if err == vfs.ENOENT {
		fs.Infof(path, "File not found")
		return 0, nil, errors.New("File not found")
	} else if err != nil {
		return 0, nil, err
	}
	if !node.IsFile() {
		return 0, nil, errors.New("Not a file")
	}

	handle, err := node.Open(os.O_RDONLY)
	if err != nil {
		return 0, nil, err
	}
	_, err = handle.Seek(offset, os.SEEK_SET)
	if err != nil {
		return 0, nil, err
	}

	// Account the transfer
	accounting.Stats.Transferring(path)
	defer accounting.Stats.DoneTransferring(path, true)

	return node.Size(), handle, nil
}

//PutFile upload a file
func (d *Driver) PutFile(path string, data io.Reader, appendData bool) (n int64, err error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	defer log.Trace(path, "append=%v", appendData)("err = %v", &err)
	var isExist bool
	node, err := d.vfs.Stat(path)
	if err == nil {
		isExist = true
		if node.IsDir() {
			return 0, errors.New("A dir has the same name")
		}
	} else {
		if os.IsNotExist(err) {
			isExist = false
		} else {
			return 0, err
		}
	}

	if appendData && !isExist {
		appendData = false
	}

	if !appendData {
		if isExist {
			err = node.Remove()
			if err != nil {
				return 0, err
			}
		}
		f, err := d.vfs.OpenFile(path, os.O_RDWR|os.O_CREATE, 0660)
		if err != nil {
			return 0, err
		}
		defer closeIO(path, f)
		bytes, err := io.Copy(f, data)
		if err != nil {
			return 0, err
		}
		return bytes, nil
	}

	of, err := d.vfs.OpenFile(path, os.O_APPEND|os.O_RDWR, 0660)
	if err != nil {
		return 0, err
	}
	defer closeIO(path, of)

	_, err = of.Seek(0, os.SEEK_END)
	if err != nil {
		return 0, err
	}

	bytes, err := io.Copy(of, data)
	if err != nil {
		return 0, err
	}

	return bytes, nil
}

//FileInfo  struct ot hold file infor for ftp server
type FileInfo struct {
	os.FileInfo

	mode  os.FileMode
	owner uint32
	group uint32
}

//Mode return êrm mode of file.
func (f *FileInfo) Mode() os.FileMode {
	return f.mode
}

//Owner return owner of file. Try to find the username if possible
func (f *FileInfo) Owner() string {
	str := fmt.Sprint(f.owner)
	u, err := user.LookupId(str)
	if err != nil {
		return str //User not found
	}
	return u.Username
}

//Group return group of file. Try to find the group name if possible
func (f *FileInfo) Group() string {
	str := fmt.Sprint(f.group)
	g, err := user.LookupGroupId(str)
	if err != nil {
		return str //Group not found default to numrical value
	}
	return g.Name
}

func closeIO(path string, c io.Closer) {
	err := c.Close()
	if err != nil {
		log.Trace(path, "")("err = %v", &err)
	}
}
