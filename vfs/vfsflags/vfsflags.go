// Package vfsflags implements command line flags to set up a vfs
package vfsflags

import (
	"github.com/ncw/rclone/fs/config/flags"
	"github.com/ncw/rclone/vfs"
	"github.com/spf13/pflag"
)

// Options set by command line flags
var (
	Opt = vfs.DefaultOpt
)

// AddFlags adds the non filing system specific flags to the command
func AddFlags(flagSet *pflag.FlagSet) {
	flags.BoolVarP(flagSet, &Opt.NoModTime, "no-modtime", "", Opt.NoModTime, "Don't read/write the modification time (can speed things up).")
	flags.BoolVarP(flagSet, &Opt.NoChecksum, "no-checksum", "", Opt.NoChecksum, "Don't compare checksums on up/download.")
	flags.BoolVarP(flagSet, &Opt.NoSeek, "no-seek", "", Opt.NoSeek, "Don't allow seeking in files.")
	flags.DurationVarP(flagSet, &Opt.DirCacheTime, "dir-cache-time", "", Opt.DirCacheTime, "Time to cache directory entries for.")
	flags.DurationVarP(flagSet, &Opt.PollInterval, "poll-interval", "", Opt.PollInterval, "Time to wait between polling for changes. Must be smaller than dir-cache-time. Only on supported remotes. Set to 0 to disable.")
	flags.BoolVarP(flagSet, &Opt.ReadOnly, "read-only", "", Opt.ReadOnly, "Mount read-only.")
	flags.FVarP(flagSet, &Opt.CacheMode, "vfs-cache-mode", "", "Cache mode off|minimal|writes|full")
	flags.DurationVarP(flagSet, &Opt.CachePollInterval, "vfs-cache-poll-interval", "", Opt.CachePollInterval, "Interval to poll the cache for stale objects.")
	flags.DurationVarP(flagSet, &Opt.CacheMaxAge, "vfs-cache-max-age", "", Opt.CacheMaxAge, "Max age of objects in the cache.")
	flags.FVarP(flagSet, &Opt.ChunkSize, "vfs-read-chunk-size", "", "Read the source objects in chunks.")
	flags.FVarP(flagSet, &Opt.ChunkSizeLimit, "vfs-read-chunk-size-limit", "", "If greater than --vfs-read-chunk-size, double the chunk size after each chunk read, until the limit is reached. 'off' is unlimited.")
	platformFlags(flagSet)
}
