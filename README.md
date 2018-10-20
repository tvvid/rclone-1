[![Logo](https://rclone.org/img/rclone-120x120.png)](https://rclone.org/)

[Website](https://rclone.org) |
[Documentation](https://rclone.org/docs/) |
[Download](https://rclone.org/downloads/) | 
[Contributing](CONTRIBUTING.md) |
[Changelog](https://rclone.org/changelog/) |
[Installation](https://rclone.org/install/) |
[Forum](https://forum.rclone.org/) | 
[G+](https://google.com/+RcloneOrg)

[![Build Status](https://travis-ci.org/ncw/rclone.svg?branch=master)](https://travis-ci.org/ncw/rclone)
[![Windows Build Status](https://ci.appveyor.com/api/projects/status/github/ncw/rclone?branch=master&passingText=windows%20-%20ok&svg=true)](https://ci.appveyor.com/project/ncw/rclone)
[![CircleCI](https://circleci.com/gh/ncw/rclone/tree/master.svg?style=svg)](https://circleci.com/gh/ncw/rclone/tree/master)
[![GoDoc](https://godoc.org/github.com/ncw/rclone?status.svg)](https://godoc.org/github.com/ncw/rclone) 

# Rclone

Rclone *("rsync for cloud storage")* is a command line program to sync files and directories to and from different cloud storage providers.

## Storage providers

  * Amazon Drive [:page_facing_up:](https://rclone.org/amazonclouddrive/) ([See note](https://rclone.org/amazonclouddrive/#status))
  * Amazon S3 [:page_facing_up:](https://rclone.org/s3/)
  * Backblaze B2 [:page_facing_up:](https://rclone.org/b2/)
  * Box [:page_facing_up:](https://rclone.org/box/)
  * Ceph [:page_facing_up:](https://rclone.org/s3/#ceph)
  * DigitalOcean Spaces [:page_facing_up:](https://rclone.org/s3/#digitalocean-spaces)
  * Dreamhost [:page_facing_up:](https://rclone.org/s3/#dreamhost)
  * Dropbox [:page_facing_up:](https://rclone.org/dropbox/)
  * FTP [:page_facing_up:](https://rclone.org/ftp/)
  * Google Cloud Storage [:page_facing_up:](https://rclone.org/googlecloudstorage/)
  * Google Drive [:page_facing_up:](https://rclone.org/drive/)
  * HTTP [:page_facing_up:](https://rclone.org/http/)
  * Hubic [:page_facing_up:](https://rclone.org/hubic/)
  * Jottacloud [:page_facing_up:](https://rclone.org/jottacloud/)
  * IBM COS S3 [:page_facing_up:](https://rclone.org/s3/#ibm-cos-s3)
  * Memset Memstore [:page_facing_up:](https://rclone.org/swift/)
  * Mega [:page_facing_up:](https://rclone.org/mega/)
  * Microsoft Azure Blob Storage [:page_facing_up:](https://rclone.org/azureblob/)
  * Microsoft OneDrive [:page_facing_up:](https://rclone.org/onedrive/)
  * Minio [:page_facing_up:](https://rclone.org/s3/#minio)
  * Nextcloud [:page_facing_up:](https://rclone.org/webdav/#nextcloud)
  * OVH [:page_facing_up:](https://rclone.org/swift/)
  * OpenDrive [:page_facing_up:](https://rclone.org/opendrive/)
  * Openstack Swift [:page_facing_up:](https://rclone.org/swift/)
  * Oracle Cloud Storage [:page_facing_up:](https://rclone.org/swift/)
  * ownCloud [:page_facing_up:](https://rclone.org/webdav/#owncloud)
  * pCloud [:page_facing_up:](https://rclone.org/pcloud/)
  * put.io [:page_facing_up:](https://rclone.org/webdav/#put-io)
  * QingStor [:page_facing_up:](https://rclone.org/qingstor/)
  * Rackspace Cloud Files [:page_facing_up:](https://rclone.org/swift/)
  * SFTP [:page_facing_up:](https://rclone.org/sftp/)
  * Wasabi [:page_facing_up:](https://rclone.org/s3/#wasabi)
  * WebDAV [:page_facing_up:](https://rclone.org/webdav/)
  * Yandex Disk [:page_facing_up:](https://rclone.org/yandex/)
  * The local filesystem [:page_facing_up:](https://rclone.org/local/)

## Features

  * MD5/SHA1 hashes checked at all times for file integrity
  * Timestamps preserved on files
  * Partial syncs supported on a whole file basis
  * [Copy](https://rclone.org/commands/rclone_copy/) mode to just copy new/changed files
  * [Sync](https://rclone.org/commands/rclone_sync/) (one way) mode to make a directory identical
  * [Check](https://rclone.org/commands/rclone_check/) mode to check for file hash equality
  * Can sync to and from network, eg two different cloud accounts
  * Optional encryption ([Crypt](https://rclone.org/crypt/))
  * Optional cache ([Cache](https://rclone.org/cache/))
  * Optional FUSE mount ([rclone mount](https://rclone.org/commands/rclone_mount/))

## Installation & documentation

Please see the rclone website for installation, usage, documentation, 
changelog and configuration walkthroughs.

  * https://rclone.org/

## Downloads

  * https://rclone.org/downloads/

License
-------

This is free software under the terms of MIT the license (check the
[COPYING file](/rclone/COPYING) included in this package).
