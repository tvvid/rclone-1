---
date: 2018-03-19T10:05:30Z
title: "rclone ncdu"
slug: rclone_ncdu
url: /commands/rclone_ncdu/
---
## rclone ncdu

Explore a remote with a text based user interface.

### Synopsis


This displays a text based user interface allowing the navigation of a
remote. It is most useful for answering the question - "What is using
all my disk space?".

<script src="https://asciinema.org/a/157793.js" id="asciicast-157793" async></script>

To make the user interface it first scans the entire remote given and
builds an in memory representation.  rclone ncdu can be used during
this scanning phase and you will see it building up the directory
structure as it goes along.

Here are the keys - press '?' to toggle the help on and off

     ↑,↓ or k,j to Move
     →,l to enter
     ←,h to return
     c toggle counts
     g toggle graph
     n,s,C sort by name,size,count
     ^L refresh screen
     ? to toggle help on and off
     q/ESC/c-C to quit

This an homage to the [ncdu tool](https://dev.yorhel.nl/ncdu) but for
rclone remotes.  It is missing lots of features at the moment, most
importantly deleting files, but is useful as it stands.


```
rclone ncdu remote:path [flags]
```

### Options

```
  -h, --help   help for ncdu
```

### Options inherited from parent commands

```
      --acd-templink-threshold int          Files >= this size will be downloaded via their tempLink. (default 9G)
      --acd-upload-wait-per-gb duration     Additional time per GB to wait after a failed complete upload to see if it appears. (default 3m0s)
      --ask-password                        Allow prompt for password for encrypted configuration. (default true)
      --auto-confirm                        If enabled, do not request console confirmation.
      --azureblob-chunk-size int            Upload chunk size. Must fit in memory. (default 4M)
      --azureblob-upload-cutoff int         Cutoff for switching to chunked upload (default 256M)
      --b2-chunk-size int                   Upload chunk size. Must fit in memory. (default 96M)
      --b2-hard-delete                      Permanently delete files on remote removal, otherwise hide files.
      --b2-test-mode string                 A flag string for X-Bz-Test-Mode header.
      --b2-upload-cutoff int                Cutoff for switching to chunked upload (default 190.735M)
      --b2-versions                         Include old versions in directory listings.
      --backup-dir string                   Make backups into hierarchy based in DIR.
      --bind string                         Local address to bind to for outgoing connections, IPv4, IPv6 or name.
      --box-upload-cutoff int               Cutoff for switching to multipart upload (default 50M)
      --buffer-size int                     Buffer size when copying files. (default 16M)
      --bwlimit BwTimetable                 Bandwidth limit in kBytes/s, or use suffix b|k|M|G or a full timetable.
      --cache-chunk-clean-interval string   Interval at which chunk cleanup runs (default "1m")
      --cache-chunk-no-memory               Disable the in-memory cache for storing chunks during streaming
      --cache-chunk-path string             Directory to cached chunk files (default "/home/ncw/.cache/rclone/cache-backend")
      --cache-chunk-size string             The size of a chunk (default "5M")
      --cache-db-path string                Directory to cache DB (default "/home/ncw/.cache/rclone/cache-backend")
      --cache-db-purge                      Purge the cache DB before
      --cache-db-wait-time duration         How long to wait for the DB to be available - 0 is unlimited (default 1s)
      --cache-dir string                    Directory rclone will use for caching. (default "/home/ncw/.cache/rclone")
      --cache-info-age string               How much time should object info be stored in cache (default "6h")
      --cache-read-retries int              How many times to retry a read from a cache storage (default 10)
      --cache-rps int                       Limits the number of requests per second to the source FS. -1 disables the rate limiter (default -1)
      --cache-tmp-upload-path string        Directory to keep temporary files until they are uploaded to the cloud storage
      --cache-tmp-wait-time string          How long should files be stored in local cache before being uploaded (default "15m")
      --cache-total-chunk-size string       The total size which the chunks can take up from the disk (default "10G")
      --cache-workers int                   How many workers should run in parallel to download chunks (default 4)
      --cache-writes                        Will cache file data on writes through the FS
      --checkers int                        Number of checkers to run in parallel. (default 8)
  -c, --checksum                            Skip based on checksum & size, not mod-time & size
      --config string                       Config file. (default "/home/ncw/.rclone.conf")
      --contimeout duration                 Connect timeout (default 1m0s)
  -L, --copy-links                          Follow symlinks and copy the pointed to item.
      --cpuprofile string                   Write cpu profile to file
      --crypt-show-mapping                  For all files listed show how the names encrypt.
      --delete-after                        When synchronizing, delete files on destination after transfering
      --delete-before                       When synchronizing, delete files on destination before transfering
      --delete-during                       When synchronizing, delete files during transfer (default)
      --delete-excluded                     Delete files on dest excluded from sync
      --disable string                      Disable a comma separated list of features.  Use help to see a list.
      --drive-auth-owner-only               Only consider files owned by the authenticated user.
      --drive-chunk-size int                Upload chunk size. Must a power of 2 >= 256k. (default 8M)
      --drive-formats string                Comma separated list of preferred formats for downloading Google docs. (default "docx,xlsx,pptx,svg")
      --drive-impersonate string            Impersonate this user when using a service account.
      --drive-list-chunk int                Size of listing chunk 100-1000. 0 to disable. (default 1000)
      --drive-shared-with-me                Only show files that are shared with me
      --drive-skip-gdocs                    Skip google documents in all listings.
      --drive-trashed-only                  Only show files that are in the trash
      --drive-upload-cutoff int             Cutoff for switching to chunked upload (default 8M)
      --drive-use-created-date              Use created date instead of modified date.
      --drive-use-trash                     Send files to the trash instead of deleting permanently. (default true)
      --dropbox-chunk-size int              Upload chunk size. Max 150M. (default 48M)
  -n, --dry-run                             Do a trial run with no permanent changes
      --dump string                         List of items to dump from: headers,bodies,requests,responses,auth,filters
      --dump-bodies                         Dump HTTP headers and bodies - may contain sensitive info
      --dump-headers                        Dump HTTP bodies - may contain sensitive info
      --exclude stringArray                 Exclude files matching pattern
      --exclude-from stringArray            Read exclude patterns from file
      --exclude-if-present string           Exclude directories if filename is present
      --fast-list                           Use recursive list if available. Uses more memory but fewer transactions.
      --files-from stringArray              Read list of source-file names from file
  -f, --filter stringArray                  Add a file-filtering rule
      --filter-from stringArray             Read filtering patterns from a file
      --gcs-location string                 Default location for buckets (us|eu|asia|us-central1|us-east1|us-east4|us-west1|asia-east1|asia-noetheast1|asia-southeast1|australia-southeast1|europe-west1|europe-west2).
      --gcs-storage-class string            Default storage class for buckets (MULTI_REGIONAL|REGIONAL|STANDARD|NEARLINE|COLDLINE|DURABLE_REDUCED_AVAILABILITY).
      --ignore-checksum                     Skip post copy check of checksums.
      --ignore-existing                     Skip all files that exist on destination
      --ignore-size                         Ignore size when skipping use mod-time or checksum.
  -I, --ignore-times                        Don't skip files that match size and time - transfer all files
      --immutable                           Do not modify files. Fail if existing files have been modified.
      --include stringArray                 Include files matching pattern
      --include-from stringArray            Read include patterns from file
      --local-no-unicode-normalization      Don't apply unicode normalization to paths and filenames
      --log-file string                     Log everything to this file
      --log-level string                    Log level DEBUG|INFO|NOTICE|ERROR (default "NOTICE")
      --low-level-retries int               Number of low level retries to do. (default 10)
      --max-age duration                    Don't transfer any file older than this in s or suffix ms|s|m|h|d|w|M|y (default off)
      --max-delete int                      When synchronizing, limit the number of deletes (default -1)
      --max-depth int                       If set limits the recursion depth to this. (default -1)
      --max-size int                        Don't transfer any file larger than this in k or suffix b|k|M|G (default off)
      --memprofile string                   Write memory profile to file
      --min-age duration                    Don't transfer any file younger than this in s or suffix ms|s|m|h|d|w|M|y (default off)
      --min-size int                        Don't transfer any file smaller than this in k or suffix b|k|M|G (default off)
      --modify-window duration              Max time diff to be considered the same (default 1ns)
      --no-check-certificate                Do not verify the server SSL certificate. Insecure.
      --no-gzip-encoding                    Don't set Accept-Encoding: gzip.
      --no-traverse                         Obsolete - does nothing.
      --no-update-modtime                   Don't update destination mod-time if files identical.
  -x, --one-file-system                     Don't cross filesystem boundaries.
      --onedrive-chunk-size int             Above this size files will be chunked - must be multiple of 320k. (default 10M)
  -q, --quiet                               Print as little stuff as possible
      --rc                                  Enable the remote control server.
      --rc-addr string                      IPaddress:Port or :Port to bind server to. (default "localhost:5572")
      --rc-cert string                      SSL PEM key (concatenation of certificate and CA certificate)
      --rc-client-ca string                 Client certificate authority to verify clients with
      --rc-htpasswd string                  htpasswd file - if not provided no authentication is done
      --rc-key string                       SSL PEM Private key
      --rc-max-header-bytes int             Maximum size of request header (default 4096)
      --rc-pass string                      Password for authentication.
      --rc-realm string                     realm for authentication (default "rclone")
      --rc-server-read-timeout duration     Timeout for server reading data (default 1h0m0s)
      --rc-server-write-timeout duration    Timeout for server writing data (default 1h0m0s)
      --rc-user string                      User name for authentication.
      --retries int                         Retry operations this many times if they fail (default 3)
      --s3-acl string                       Canned ACL used when creating buckets and/or storing objects in S3
      --s3-storage-class string             Storage class to use when uploading S3 objects (STANDARD|REDUCED_REDUNDANCY|STANDARD_IA)
      --sftp-ask-password                   Allow asking for SFTP password when needed.
      --size-only                           Skip based on size only, not mod-time or checksum
      --skip-links                          Don't warn about skipped symlinks.
      --stats duration                      Interval between printing stats, e.g 500ms, 60s, 5m. (0 to disable) (default 1m0s)
      --stats-file-name-length int          Max file name length in stats. 0 for no limit (default 40)
      --stats-log-level string              Log level to show --stats output DEBUG|INFO|NOTICE|ERROR (default "INFO")
      --stats-unit string                   Show data rate in stats as either 'bits' or 'bytes'/s (default "bytes")
      --streaming-upload-cutoff int         Cutoff for switching to chunked upload if file size is unknown. Upload starts after reaching cutoff or when file ends. (default 100k)
      --suffix string                       Suffix for use with --backup-dir.
      --swift-chunk-size int                Above this size files will be chunked into a _segments container. (default 5G)
      --syslog                              Use Syslog for logging
      --syslog-facility string              Facility for syslog, eg KERN,USER,... (default "DAEMON")
      --timeout duration                    IO idle timeout (default 5m0s)
      --tpslimit float                      Limit HTTP transactions per second to this.
      --tpslimit-burst int                  Max burst of transactions for --tpslimit. (default 1)
      --track-renames                       When synchronizing, track file renames and do a server side move if possible
      --transfers int                       Number of file transfers to run in parallel. (default 4)
  -u, --update                              Skip files that are newer on the destination.
      --user-agent string                   Set the user-agent to a specified string. The default is rclone/ version (default "rclone/v1.40")
  -v, --verbose count                       Print lots more stuff (repeat for more)
```

### SEE ALSO

* [rclone](/commands/rclone/)	 - Sync files and directories to and from local and remote object stores - v1.40

###### Auto generated by spf13/cobra on 19-Mar-2018
