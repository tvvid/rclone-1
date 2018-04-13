SHELL = bash
TAG := $(shell echo $$(git describe --abbrev=8 --tags)-$${APPVEYOR_REPO_BRANCH:-$${TRAVIS_BRANCH:-$$(git rev-parse --abbrev-ref HEAD)}} | sed 's/-\([0-9]\)-/-00\1-/; s/-\([0-9][0-9]\)-/-0\1-/; s/-\(HEAD\|master\)$$//')
LAST_TAG := $(shell git describe --tags --abbrev=0)
NEW_TAG := $(shell echo $(LAST_TAG) | perl -lpe 's/v//; $$_ += 0.01; $$_ = sprintf("v%.2f", $$_)')
GO_VERSION := $(shell go version)
GO_FILES := $(shell go list ./... | grep -v /vendor/ )
# Run full tests if go >= go1.9
FULL_TESTS := $(shell go version | perl -lne 'print "go$$1.$$2" if /go(\d+)\.(\d+)/ && ($$1 > 1 || $$2 >= 9)')
BETA_URL := https://beta.rclone.org/$(TAG)/
# Pass in GOTAGS=xyz on the make command line to set build tags
ifdef GOTAGS
BUILDTAGS=-tags "$(GOTAGS)"
endif

.PHONY: rclone vars version

rclone:
	touch fs/version.go
	go install -v --ldflags "-s -X github.com/ncw/rclone/fs.Version=$(TAG)" $(BUILDTAGS)
	cp -av `go env GOPATH`/bin/rclone .

vars:
	@echo SHELL="'$(SHELL)'"
	@echo TAG="'$(TAG)'"
	@echo LAST_TAG="'$(LAST_TAG)'"
	@echo NEW_TAG="'$(NEW_TAG)'"
	@echo GO_VERSION="'$(GO_VERSION)'"
	@echo FULL_TESTS="'$(FULL_TESTS)'"
	@echo BETA_URL="'$(BETA_URL)'"

version:
	@echo '$(TAG)'

# Full suite of integration tests
test:	rclone
	go install github.com/ncw/rclone/fstest/test_all
	-go test -v -count 1 $(BUILDTAGS) $(GO_FILES) 2>&1 | tee test.log
	-test_all github.com/ncw/rclone/fs/operations github.com/ncw/rclone/fs/sync 2>&1 | tee fs/test_all.log
	@echo "Written logs in test.log and fs/test_all.log"

# Quick test
quicktest:
	RCLONE_CONFIG="/notfound" go test $(BUILDTAGS) $(GO_FILES)
ifdef FULL_TESTS
	RCLONE_CONFIG="/notfound" go test $(BUILDTAGS) -cpu=2 -race $(GO_FILES)
endif

# Do source code quality checks
check:	rclone
ifdef FULL_TESTS
	go vet $(BUILDTAGS) -printfuncs Debugf,Infof,Logf,Errorf ./...
	errcheck $(BUILDTAGS) ./...
	find . -name \*.go | grep -v /vendor/ | xargs goimports -d | grep . ; test $$? -eq 1
	go list ./... | xargs -n1 golint | grep -E -v '(StorageUrl|CdnUrl)' ; test $$? -eq 1
else
	@echo Skipping source quality tests as version of go too old
endif

# Get the build dependencies
build_dep:
ifdef FULL_TESTS
	go get -u github.com/kisielk/errcheck
	go get -u golang.org/x/tools/cmd/goimports
	go get -u github.com/golang/lint/golint
	go get -u github.com/inconshreveable/mousetrap
	go get -u github.com/tools/godep
endif

# Get the release dependencies
release_dep:
	go get -u github.com/goreleaser/nfpm

# Update dependencies
update:
	go get -u github.com/golang/dep/cmd/dep
	dep ensure -update -v

doc:	rclone.1 MANUAL.html MANUAL.txt

rclone.1:	MANUAL.md
	pandoc -s --from markdown --to man MANUAL.md -o rclone.1

MANUAL.md:	bin/make_manual.py docs/content/*.md commanddocs
	./bin/make_manual.py

MANUAL.html:	MANUAL.md
	pandoc -s --from markdown --to html MANUAL.md -o MANUAL.html

MANUAL.txt:	MANUAL.md
	pandoc -s --from markdown --to plain MANUAL.md -o MANUAL.txt

commanddocs: rclone
	rclone gendocs docs/content/commands/

install: rclone
	install -d ${DESTDIR}/usr/bin
	install -t ${DESTDIR}/usr/bin ${GOPATH}/bin/rclone

clean:
	go clean ./...
	find . -name \*~ | xargs -r rm -f
	rm -rf build docs/public
	rm -f rclone fs/operations/operations.test fs/sync/sync.test fs/test_all.log test.log

website:
	cd docs && hugo

upload_website:	website
	rclone -v sync docs/public memstore:www-rclone-org

tarball:
	git archive -9 --format=tar.gz --prefix=rclone-$(TAG)/ -o build/rclone-$(TAG).tar.gz $(TAG)

sign_upload:
	cd build && md5sum rclone-* | gpg --clearsign > MD5SUMS
	cd build && sha1sum rclone-* | gpg --clearsign > SHA1SUMS
	cd build && sha256sum rclone-* | gpg --clearsign > SHA256SUMS

check_sign:
	cd build && gpg --verify MD5SUMS && gpg --decrypt MD5SUMS | md5sum -c
	cd build && gpg --verify SHA1SUMS && gpg --decrypt SHA1SUMS | sha1sum -c
	cd build && gpg --verify SHA256SUMS && gpg --decrypt SHA256SUMS | sha256sum -c

upload:
	rclone -v copy --exclude '*current*' build/ memstore:downloads-rclone-org/$(TAG)
	rclone -v copy --include '*current*' --include version.txt build/ memstore:downloads-rclone-org

upload_github:
	./bin/upload-github $(TAG)

cross:	doc
	go run bin/cross-compile.go -release current $(BUILDTAGS) $(TAG)

beta:
	go run bin/cross-compile.go $(BUILDTAGS) $(TAG)β
	rclone -v copy build/ memstore:pub-rclone-org/$(TAG)β
	@echo Beta release ready at https://pub.rclone.org/$(TAG)%CE%B2/

log_since_last_release:
	git log $(LAST_TAG)..

compile_all:
ifdef FULL_TESTS
	go run bin/cross-compile.go -parallel 8 -compile-only $(BUILDTAGS) $(TAG)β
else
	@echo Skipping compile all as version of go too old
endif

appveyor_upload:
	rclone --config bin/travis.rclone.conf -v copy --exclude '*beta-latest*' build/ memstore:beta-rclone-org/$(TAG)
ifeq ($(APPVEYOR_REPO_BRANCH),master)
	rclone --config bin/travis.rclone.conf -v copy --include '*beta-latest*' --include version.txt build/ memstore:beta-rclone-org
endif
	@echo Beta release ready at $(BETA_URL)

travis_beta:
	go run bin/get-github-release.go -extract nfpm goreleaser/nfpm 'nfpm_.*_Linux_x86_64.tar.gz'
	git log $(LAST_TAG).. > /tmp/git-log.txt
	go run bin/cross-compile.go -release beta-latest -git-log /tmp/git-log.txt -exclude "^windows/" -parallel 8 $(BUILDTAGS) $(TAG)β
	rclone --config bin/travis.rclone.conf -v copy --exclude '*beta-latest*' build/ memstore:beta-rclone-org/$(TAG)
ifeq ($(TRAVIS_BRANCH),master)
	rclone --config bin/travis.rclone.conf -v copy --include '*beta-latest*' --include version.txt build/ memstore:beta-rclone-org
endif
	@echo Beta release ready at $(BETA_URL)

# Fetch the windows builds from appveyor
fetch_windows:
	rclone -v copy --include 'rclone-v*-windows-*.zip' memstore:beta-rclone-org/$(TAG) build/
	-#cp -av build/rclone-v*-windows-386.zip build/rclone-current-windows-386.zip
	-#cp -av build/rclone-v*-windows-amd64.zip build/rclone-current-windows-amd64.zip
	md5sum build/rclone-*-windows-*.zip | sort

serve:	website
	cd docs && hugo server -v -w

tag:	doc
	@echo "Old tag is $(LAST_TAG)"
	@echo "New tag is $(NEW_TAG)"
	echo -e "package fs\n\n// Version of rclone\nvar Version = \"$(NEW_TAG)\"\n" | gofmt > fs/version.go
	echo -n "$(NEW_TAG)" > docs/layouts/partials/version.html
	git tag -s -m "Version $(NEW_TAG)" $(NEW_TAG)
	@echo "Edit the new changelog in docs/content/changelog.md"
	@echo "  * $(NEW_TAG) -" `date -I` >> docs/content/changelog.md
	@git log $(LAST_TAG)..$(NEW_TAG) --oneline >> docs/content/changelog.md
	@echo "Then commit the changes"
	@echo git commit -m \"Version $(NEW_TAG)\" -a -v
	@echo "And finally run make retag before make cross etc"

retag:
	git tag -f -s -m "Version $(LAST_TAG)" $(LAST_TAG)

startdev:
	echo -e "package fs\n\n// Version of rclone\nvar Version = \"$(LAST_TAG)-DEV\"\n" | gofmt > fs/version.go
	git commit -m "Start $(LAST_TAG)-DEV development" fs/version.go

winzip:
	zip -9 rclone-$(TAG).zip rclone.exe

