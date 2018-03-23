Extra required software for making a release
  * [github-release](https://github.com/aktau/github-release) for uploading packages
  * pandoc for making the html and man pages

Making a release
  * git status - make sure everything is checked in
  * Check travis & appveyor builds are green
  * make check
  * make test # see integration test server or run locally
  * make tag
  * edit docs/content/changelog.md
  * make doc
  * git status - to check for new man pages - git add them
  * git commit -a -v -m "Version v1.XX"
  * make retag
  * make release_dep
  * # Set the GOPATH for a current stable go compiler
  * make cross
  * git checkout docs/content/commands # to undo date changes in commands
  * git push --tags origin master
  * git push --tags origin master:stable # update the stable branch for packager.io
  * # Wait for the appveyor and travis builds to complete then fetch the windows binaries from appveyor
  * make fetch_windows
  * make tarball
  * make sign_upload
  * make check_sign
  * make upload
  * make upload_website
  * make upload_github
  * make startdev
  * # announce with forum post, twitter post, G+ post

Early in the next release cycle update the vendored dependencies
  * make update
  * git status
  * git add new files
  * carry forward any patches to vendor stuff
  * git commit -a -v

Make the version number be just in a file?
