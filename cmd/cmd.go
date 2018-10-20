// Package cmd implemnts the rclone command
//
// It is in a sub package so it's internals can be re-used elsewhere
package cmd

// FIXME only attach the remote flags when using a remote???
// would probably mean bringing all the flags in to here? Or define some flagsets in fs...

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/accounting"
	"github.com/ncw/rclone/fs/config/configflags"
	"github.com/ncw/rclone/fs/config/flags"
	"github.com/ncw/rclone/fs/filter"
	"github.com/ncw/rclone/fs/filter/filterflags"
	"github.com/ncw/rclone/fs/fserrors"
	"github.com/ncw/rclone/fs/fspath"
	fslog "github.com/ncw/rclone/fs/log"
	"github.com/ncw/rclone/fs/rc"
	"github.com/ncw/rclone/fs/rc/rcflags"
	"github.com/ncw/rclone/lib/atexit"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Globals
var (
	// Flags
	cpuProfile      = flags.StringP("cpuprofile", "", "", "Write cpu profile to file")
	memProfile      = flags.StringP("memprofile", "", "", "Write memory profile to file")
	statsInterval   = flags.DurationP("stats", "", time.Minute*1, "Interval between printing stats, e.g 500ms, 60s, 5m. (0 to disable)")
	dataRateUnit    = flags.StringP("stats-unit", "", "bytes", "Show data rate in stats as either 'bits' or 'bytes'/s")
	version         bool
	retries         = flags.IntP("retries", "", 3, "Retry operations this many times if they fail")
	retriesInterval = flags.DurationP("retries-sleep", "", 0, "Interval between retrying operations if they fail, e.g 500ms, 60s, 5m. (0 to disable)")
	// Errors
	errorCommandNotFound    = errors.New("command not found")
	errorUncategorized      = errors.New("uncategorized error")
	errorNotEnoughArguments = errors.New("not enough arguments")
	errorTooManyArguents    = errors.New("too many arguments")
)

const (
	exitCodeSuccess = iota
	exitCodeUsageError
	exitCodeUncategorizedError
	exitCodeDirNotFound
	exitCodeFileNotFound
	exitCodeRetryError
	exitCodeNoRetryError
	exitCodeFatalError
	exitCodeTransferExceeded
)

// ShowVersion prints the version to stdout
func ShowVersion() {
	fmt.Printf("rclone %s\n", fs.Version)
	fmt.Printf("- os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("- go version: %s\n", runtime.Version())
}

// NewFsFile creates a Fs from a name but may point to a file.
//
// It returns a string with the file name if points to a file
// otherwise "".
func NewFsFile(remote string) (fs.Fs, string) {
	_, _, fsPath, err := fs.ParseRemote(remote)
	if err != nil {
		fs.CountError(err)
		log.Fatalf("Failed to create file system for %q: %v", remote, err)
	}
	f, err := fs.NewFs(remote)
	switch err {
	case fs.ErrorIsFile:
		return f, path.Base(fsPath)
	case nil:
		return f, ""
	default:
		fs.CountError(err)
		log.Fatalf("Failed to create file system for %q: %v", remote, err)
	}
	return nil, ""
}

// newFsFileAddFilter creates a src Fs from a name
//
// This works the same as NewFsFile however it adds filters to the Fs
// to limit it to a single file if the remote pointed to a file.
func newFsFileAddFilter(remote string) (fs.Fs, string) {
	f, fileName := NewFsFile(remote)
	if fileName != "" {
		if !filter.Active.InActive() {
			err := errors.Errorf("Can't limit to single files when using filters: %v", remote)
			fs.CountError(err)
			log.Fatalf(err.Error())
		}
		// Limit transfers to this file
		err := filter.Active.AddFile(fileName)
		if err != nil {
			fs.CountError(err)
			log.Fatalf("Failed to limit to single file %q: %v", remote, err)
		}
	}
	return f, fileName
}

// NewFsSrc creates a new src fs from the arguments.
//
// The source can be a file or a directory - if a file then it will
// limit the Fs to a single file.
func NewFsSrc(args []string) fs.Fs {
	fsrc, _ := newFsFileAddFilter(args[0])
	return fsrc
}

// newFsDir creates an Fs from a name
//
// This must point to a directory
func newFsDir(remote string) fs.Fs {
	f, err := fs.NewFs(remote)
	if err != nil {
		fs.CountError(err)
		log.Fatalf("Failed to create file system for %q: %v", remote, err)
	}
	return f
}

// NewFsDir creates a new Fs from the arguments
//
// The argument must point a directory
func NewFsDir(args []string) fs.Fs {
	fdst := newFsDir(args[0])
	return fdst
}

// NewFsSrcDst creates a new src and dst fs from the arguments
func NewFsSrcDst(args []string) (fs.Fs, fs.Fs) {
	fsrc, _ := newFsFileAddFilter(args[0])
	fdst := newFsDir(args[1])
	return fsrc, fdst
}

// NewFsSrcFileDst creates a new src and dst fs from the arguments
//
// The source may be a file, in which case the source Fs and file name is returned
func NewFsSrcFileDst(args []string) (fsrc fs.Fs, srcFileName string, fdst fs.Fs) {
	fsrc, srcFileName = NewFsFile(args[0])
	fdst = newFsDir(args[1])
	return fsrc, srcFileName, fdst
}

// NewFsSrcDstFiles creates a new src and dst fs from the arguments
// If src is a file then srcFileName and dstFileName will be non-empty
func NewFsSrcDstFiles(args []string) (fsrc fs.Fs, srcFileName string, fdst fs.Fs, dstFileName string) {
	fsrc, srcFileName = newFsFileAddFilter(args[0])
	// If copying a file...
	dstRemote := args[1]
	// If file exists then srcFileName != "", however if the file
	// doesn't exist then we assume it is a directory...
	if srcFileName != "" {
		dstRemote, dstFileName = fspath.Split(dstRemote)
		if dstRemote == "" {
			dstRemote = "."
		}
		if dstFileName == "" {
			log.Fatalf("%q is a directory", args[1])
		}
	}
	fdst, err := fs.NewFs(dstRemote)
	switch err {
	case fs.ErrorIsFile:
		fs.CountError(err)
		log.Fatalf("Source doesn't exist or is a directory and destination is a file")
	case nil:
	default:
		fs.CountError(err)
		log.Fatalf("Failed to create file system for destination %q: %v", dstRemote, err)
	}
	return
}

// NewFsDstFile creates a new dst fs with a destination file name from the arguments
func NewFsDstFile(args []string) (fdst fs.Fs, dstFileName string) {
	dstRemote, dstFileName := fspath.Split(args[0])
	if dstRemote == "" {
		dstRemote = "."
	}
	if dstFileName == "" {
		log.Fatalf("%q is a directory", args[0])
	}
	fdst = newFsDir(dstRemote)
	return
}

// ShowStats returns true if the user added a `--stats` flag to the command line.
//
// This is called by Run to override the default value of the
// showStats passed in.
func ShowStats() bool {
	statsIntervalFlag := pflag.Lookup("stats")
	return statsIntervalFlag != nil && statsIntervalFlag.Changed
}

// Run the function with stats and retries if required
func Run(Retry bool, showStats bool, cmd *cobra.Command, f func() error) {
	var err error
	stopStats := func() {}
	if !showStats && ShowStats() {
		showStats = true
	}
	if fs.Config.Progress {
		stopStats = startProgress()
	} else if showStats {
		stopStats = StartStats()
	}
	SigInfoHandler()
	for try := 1; try <= *retries; try++ {
		err = f()
		if !Retry || (err == nil && !accounting.Stats.Errored()) {
			if try > 1 {
				fs.Errorf(nil, "Attempt %d/%d succeeded", try, *retries)
			}
			break
		}
		if fserrors.IsFatalError(err) || accounting.Stats.HadFatalError() {
			fs.Errorf(nil, "Fatal error received - not attempting retries")
			break
		}
		if fserrors.IsNoRetryError(err) || (accounting.Stats.Errored() && !accounting.Stats.HadRetryError()) {
			fs.Errorf(nil, "Can't retry this error - not attempting retries")
			break
		}
		if err != nil {
			fs.Errorf(nil, "Attempt %d/%d failed with %d errors and: %v", try, *retries, accounting.Stats.GetErrors(), err)
		} else {
			fs.Errorf(nil, "Attempt %d/%d failed with %d errors", try, *retries, accounting.Stats.GetErrors())
		}
		if try < *retries {
			accounting.Stats.ResetErrors()
		}
		if *retriesInterval > 0 {
			time.Sleep(*retriesInterval)
		}
	}
	stopStats()
	if err != nil {
		log.Printf("Failed to %s: %v", cmd.Name(), err)
		resolveExitCode(err)
	}
	if showStats && (accounting.Stats.Errored() || *statsInterval > 0) {
		accounting.Stats.Log()
	}
	fs.Debugf(nil, "%d go routines active\n", runtime.NumGoroutine())

	// dump all running go-routines
	if fs.Config.Dump&fs.DumpGoRoutines != 0 {
		err = pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
		if err != nil {
			fs.Errorf(nil, "Failed to dump goroutines: %v", err)
		}
	}

	// dump open files
	if fs.Config.Dump&fs.DumpOpenFiles != 0 {
		c := exec.Command("lsof", "-p", strconv.Itoa(os.Getpid()))
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		err = c.Run()
		if err != nil {
			fs.Errorf(nil, "Failed to list open files: %v", err)
		}
	}

	if accounting.Stats.Errored() {
		resolveExitCode(accounting.Stats.GetLastError())
	}
}

// CheckArgs checks there are enough arguments and prints a message if not
func CheckArgs(MinArgs, MaxArgs int, cmd *cobra.Command, args []string) {
	if len(args) < MinArgs {
		_ = cmd.Usage()
		_, _ = fmt.Fprintf(os.Stderr, "Command %s needs %d arguments minimum\n", cmd.Name(), MinArgs)
		// os.Exit(1)
		resolveExitCode(errorNotEnoughArguments)
	} else if len(args) > MaxArgs {
		_ = cmd.Usage()
		_, _ = fmt.Fprintf(os.Stderr, "Command %s needs %d arguments maximum\n", cmd.Name(), MaxArgs)
		// os.Exit(1)
		resolveExitCode(errorTooManyArguents)
	}
}

// StartStats prints the stats every statsInterval
//
// It returns a func which should be called to stop the stats.
func StartStats() func() {
	if *statsInterval <= 0 {
		return func() {}
	}
	stopStats := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(*statsInterval)
		for {
			select {
			case <-ticker.C:
				accounting.Stats.Log()
			case <-stopStats:
				ticker.Stop()
				return
			}
		}
	}()
	return func() {
		close(stopStats)
		wg.Wait()
	}
}

// initConfig is run by cobra after initialising the flags
func initConfig() {
	// Start the logger
	fslog.InitLogging()

	// Finish parsing any command line flags
	configflags.SetFlags()

	// Load filters
	var err error
	filter.Active, err = filter.NewFilter(&filterflags.Opt)
	if err != nil {
		log.Fatalf("Failed to load filters: %v", err)
	}

	// Write the args for debug purposes
	fs.Debugf("rclone", "Version %q starting with parameters %q", fs.Version, os.Args)

	// Start the remote control if configured
	rc.Start(&rcflags.Opt)

	// Setup CPU profiling if desired
	if *cpuProfile != "" {
		fs.Infof(nil, "Creating CPU profile %q\n", *cpuProfile)
		f, err := os.Create(*cpuProfile)
		if err != nil {
			fs.CountError(err)
			log.Fatal(err)
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			fs.CountError(err)
			log.Fatal(err)
		}
		atexit.Register(func() {
			pprof.StopCPUProfile()
		})
	}

	// Setup memory profiling if desired
	if *memProfile != "" {
		atexit.Register(func() {
			fs.Infof(nil, "Saving Memory profile %q\n", *memProfile)
			f, err := os.Create(*memProfile)
			if err != nil {
				fs.CountError(err)
				log.Fatal(err)
			}
			err = pprof.WriteHeapProfile(f)
			if err != nil {
				fs.CountError(err)
				log.Fatal(err)
			}
			err = f.Close()
			if err != nil {
				fs.CountError(err)
				log.Fatal(err)
			}
		})
	}

	if m, _ := regexp.MatchString("^(bits|bytes)$", *dataRateUnit); m == false {
		fs.Errorf(nil, "Invalid unit passed to --stats-unit. Defaulting to bytes.")
		fs.Config.DataRateUnit = "bytes"
	} else {
		fs.Config.DataRateUnit = *dataRateUnit
	}
}

func resolveExitCode(err error) {
	atexit.Run()
	if err == nil {
		os.Exit(exitCodeSuccess)
	}

	_, unwrapped := fserrors.Cause(err)

	switch {
	case unwrapped == fs.ErrorDirNotFound:
		os.Exit(exitCodeDirNotFound)
	case unwrapped == fs.ErrorObjectNotFound:
		os.Exit(exitCodeFileNotFound)
	case unwrapped == errorUncategorized:
		os.Exit(exitCodeUncategorizedError)
	case unwrapped == accounting.ErrorMaxTransferLimitReached:
		os.Exit(exitCodeTransferExceeded)
	case fserrors.ShouldRetry(err):
		os.Exit(exitCodeRetryError)
	case fserrors.IsNoRetryError(err):
		os.Exit(exitCodeNoRetryError)
	case fserrors.IsFatalError(err):
		os.Exit(exitCodeFatalError)
	default:
		os.Exit(exitCodeUsageError)
	}
}

var backendFlags map[string]struct{}

// AddBackendFlags creates flags for all the backend options
func AddBackendFlags() {
	backendFlags = map[string]struct{}{}
	for _, fsInfo := range fs.Registry {
		done := map[string]struct{}{}
		for i := range fsInfo.Options {
			opt := &fsInfo.Options[i]
			// Skip if done already (eg with Provider options)
			if _, doneAlready := done[opt.Name]; doneAlready {
				continue
			}
			done[opt.Name] = struct{}{}
			// Make a flag from each option
			name := opt.FlagName(fsInfo.Prefix)
			found := pflag.CommandLine.Lookup(name) != nil
			if !found {
				// Take first line of help only
				help := strings.TrimSpace(opt.Help)
				if nl := strings.IndexRune(help, '\n'); nl >= 0 {
					help = help[:nl]
				}
				help = strings.TrimSpace(help)
				flag := pflag.CommandLine.VarPF(opt, name, string(opt.ShortOpt), help)
				if _, isBool := opt.Default.(bool); isBool {
					flag.NoOptDefVal = "true"
				}
				// Hide on the command line if requested
				if opt.Hide&fs.OptionHideCommandLine != 0 {
					flag.Hidden = true
				}
				backendFlags[name] = struct{}{}
			} else {
				fs.Errorf(nil, "Not adding duplicate flag --%s", name)
			}
			//flag.Hidden = true
		}
	}
}

// Main runs rclone interpreting flags and commands out of os.Args
func Main() {
	setupRootCommand(Root)
	AddBackendFlags()
	if err := Root.Execute(); err != nil {
		log.Fatalf("Fatal error: %v", err)
	}
}
