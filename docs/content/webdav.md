---
title: "WebDAV"
description: "Rclone docs for WebDAV"
date: "2017-10-01"
---

<i class="fa fa-globe"></i> WebDAV
-----------------------------------------

Paths are specified as `remote:path`

Paths may be as deep as required, eg `remote:directory/subdirectory`.

To configure the WebDAV remote you will need to have a URL for it, and
a username and password.  If you know what kind of system you are
connecting to then rclone can enable extra features.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
 1 / Amazon Drive
   \ "amazon cloud drive"
 2 / Amazon S3 (also Dreamhost, Ceph, Minio)
   \ "s3"
 3 / Backblaze B2
   \ "b2"
 4 / Box
   \ "box"
 5 / Dropbox
   \ "dropbox"
 6 / Encrypt/Decrypt a remote
   \ "crypt"
 7 / FTP Connection
   \ "ftp"
 8 / Google Cloud Storage (this is not Google Drive)
   \ "google cloud storage"
 9 / Google Drive
   \ "drive"
10 / Hubic
   \ "hubic"
11 / Local Disk
   \ "local"
12 / Microsoft Azure Blob Storage
   \ "azureblob"
13 / Microsoft OneDrive
   \ "onedrive"
14 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
   \ "swift"
15 / Pcloud
   \ "pcloud"
16 / QingCloud Object Storage
   \ "qingstor"
17 / SSH/SFTP Connection
   \ "sftp"
18 / WebDAV
   \ "webdav"
19 / Yandex Disk
   \ "yandex"
20 / http Connection
   \ "http"
Storage> webdav
URL of http host to connect to
Choose a number from below, or type in your own value
 1 / Connect to example.com
   \ "https://example.com"
url> https://example.com/remote.php/webdav/
Name of the WebDAV site/service/software you are using
Choose a number from below, or type in your own value
 1 / Nextcloud
   \ "nextcloud"
 2 / Owncloud
   \ "owncloud"
 3 / Sharepoint
   \ "sharepoint"
 4 / Other site/service or software
   \ "other"
vendor> 1
User name
user> user
Password.
y) Yes type in my own password
g) Generate random password
n) No leave this optional password blank
y/g/n> y
Enter the password:
password:
Confirm the password:
password:
Remote config
--------------------
[remote]
url = https://example.com/remote.php/webdav/
vendor = nextcloud
user = user
pass = *** ENCRYPTED ***
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

Once configured you can then use `rclone` like this,

List directories in top level of your WebDAV

    rclone lsd remote:

List all the files in your WebDAV

    rclone ls remote:

To copy a local directory to an WebDAV directory called backup

    rclone copy /home/source remote:backup

### Modified time and hashes ###

Plain WebDAV does not support modified times.  However when used with
Owncloud or Nextcloud rclone will support modified times.

Hashes are not supported.

### Owncloud ###

Click on the settings cog in the bottom right of the page and this
will show the WebDAV URL that rclone needs in the config step.  It
will look something like `https://example.com/remote.php/webdav/`.

Owncloud supports modified times using the `X-OC-Mtime` header.

### Nextcloud ###

This is configured in an identical way to Owncloud.  Note that
Nextcloud does not support streaming of files (`rcat`) whereas
Owncloud does. This [may be
fixed](https://github.com/nextcloud/nextcloud-snap/issues/365) in the
future.

## Put.io ##

put.io can be accessed in a read only way using webdav.

Configure the `url` as `https://webdav.put.io` and use your normal
account username and password for `user` and `pass`.  Set the `vendor`
to `other`.

Your config file should end up looking like this:

```
[putio]
type = webdav
url = https://webdav.put.io
vendor = other
user = YourUserName
pass = encryptedpassword
```

If you are using `put.io` with `rclone mount` then use the
`--read-only` flag to signal to the OS that it can't write to the
mount.

For more help see [the put.io webdav docs](http://help.put.io/apps-and-integrations/ftp-and-webdav).

## Sharepoint ##

Can be used with Sharepoint provided by OneDrive for Business
or Office365 Education Accounts.
This feature is only needed for a few of these Accounts,
mostly Office365 Education ones. These accounts are sometimes not
verified by the domain owner [github#1975](https://github.com/ncw/rclone/issues/1975)

This means that these accounts can't be added using the official
API (other Accounts should work with the "onedrive" option). However,
it is possible to access them using webdav.

To use a sharepoint remote with rclone, add it like this:
First, you need to get your remote's URL:

- Go [here](https://onedrive.live.com/about/en-us/signin/)
  to open your OneDrive or to sign in
- Now take a look at your address bar, the URL should look like this:
  `https://[YOUR-DOMAIN]-my.sharepoint.com/personal/[YOUR-EMAIL]/_layouts/15/onedrive.aspx`

You'll only need this URL upto the email address. After that, you'll
most likely want to add "/Documents". That subdirectory contains
the actual data stored on your OneDrive.

Add the remote to rclone like this:
Configure the `url` as `https://[YOUR-DOMAIN]-my.sharepoint.com/personal/[YOUR-EMAIL]/Documents`
and use your normal account email and password for `user` and `pass`.
If you have 2FA enabled, you have to generate an app password.
Set the `vendor` to `sharepoint`.

Your config file should look like this:

```
[sharepoint]
type = webdav
url = https://[YOUR-DOMAIN]-my.sharepoint.com/personal/[YOUR-EMAIL]/Documents
vendor = other
user = YourEmailAddress
pass = encryptedpassword
```