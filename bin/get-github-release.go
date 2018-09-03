// +build ignore

// Get the latest release from a github project
//
// If GITHUB_USER and GITHUB_TOKEN are set then these will be used to
// authenticate the request which is useful to avoid rate limits.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

var (
	// Flags
	install = flag.Bool("install", false, "Install the downloaded package using sudo dpkg -i.")
	extract = flag.String("extract", "", "Extract the named executable from the .tar.gz and install into bindir.")
	bindir  = flag.String("bindir", defaultBinDir(), "Directory to install files downloaded with -extract.")
	// Globals
	matchProject = regexp.MustCompile(`^(\w+)/(\w+)$`)
)

// A github release
//
// Made by pasting the JSON into https://mholt.github.io/json-to-go/
type Release struct {
	URL             string `json:"url"`
	AssetsURL       string `json:"assets_url"`
	UploadURL       string `json:"upload_url"`
	HTMLURL         string `json:"html_url"`
	ID              int    `json:"id"`
	TagName         string `json:"tag_name"`
	TargetCommitish string `json:"target_commitish"`
	Name            string `json:"name"`
	Draft           bool   `json:"draft"`
	Author          struct {
		Login             string `json:"login"`
		ID                int    `json:"id"`
		AvatarURL         string `json:"avatar_url"`
		GravatarID        string `json:"gravatar_id"`
		URL               string `json:"url"`
		HTMLURL           string `json:"html_url"`
		FollowersURL      string `json:"followers_url"`
		FollowingURL      string `json:"following_url"`
		GistsURL          string `json:"gists_url"`
		StarredURL        string `json:"starred_url"`
		SubscriptionsURL  string `json:"subscriptions_url"`
		OrganizationsURL  string `json:"organizations_url"`
		ReposURL          string `json:"repos_url"`
		EventsURL         string `json:"events_url"`
		ReceivedEventsURL string `json:"received_events_url"`
		Type              string `json:"type"`
		SiteAdmin         bool   `json:"site_admin"`
	} `json:"author"`
	Prerelease  bool      `json:"prerelease"`
	CreatedAt   time.Time `json:"created_at"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []struct {
		URL      string `json:"url"`
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Label    string `json:"label"`
		Uploader struct {
			Login             string `json:"login"`
			ID                int    `json:"id"`
			AvatarURL         string `json:"avatar_url"`
			GravatarID        string `json:"gravatar_id"`
			URL               string `json:"url"`
			HTMLURL           string `json:"html_url"`
			FollowersURL      string `json:"followers_url"`
			FollowingURL      string `json:"following_url"`
			GistsURL          string `json:"gists_url"`
			StarredURL        string `json:"starred_url"`
			SubscriptionsURL  string `json:"subscriptions_url"`
			OrganizationsURL  string `json:"organizations_url"`
			ReposURL          string `json:"repos_url"`
			EventsURL         string `json:"events_url"`
			ReceivedEventsURL string `json:"received_events_url"`
			Type              string `json:"type"`
			SiteAdmin         bool   `json:"site_admin"`
		} `json:"uploader"`
		ContentType        string    `json:"content_type"`
		State              string    `json:"state"`
		Size               int       `json:"size"`
		DownloadCount      int       `json:"download_count"`
		CreatedAt          time.Time `json:"created_at"`
		UpdatedAt          time.Time `json:"updated_at"`
		BrowserDownloadURL string    `json:"browser_download_url"`
	} `json:"assets"`
	TarballURL string `json:"tarball_url"`
	ZipballURL string `json:"zipball_url"`
	Body       string `json:"body"`
}

// checks if a path has write access
func writable(path string) bool {
	return unix.Access(path, unix.W_OK) == nil
}

// Directory to install releases in by default
//
// Find writable directories on $PATH.  Use the first writable
// directory which is in $HOME or failing that the first writable
// directory.
//
// Returns "" if none of the above were found
func defaultBinDir() string {
	home := os.Getenv("HOME")
	var binDir string
	for _, dir := range strings.Split(os.Getenv("PATH"), ":") {
		if writable(dir) {
			if strings.HasPrefix(dir, home) {
				return dir
			}
			if binDir != "" {
				binDir = dir
			}
		}
	}
	return binDir
}

// read the body or an error message
func readBody(in io.Reader) string {
	data, err := ioutil.ReadAll(in)
	if err != nil {
		return fmt.Sprintf("Error reading body: %v", err.Error())
	}
	return string(data)
}

// Get an asset URL and name
func getAsset(project string, matchName *regexp.Regexp) (string, string) {
	url := "https://api.github.com/repos/" + project + "/releases/latest"
	log.Printf("Fetching asset info for %q from %q", project, url)
	user, pass := os.Getenv("GITHUB_USER"), os.Getenv("GITHUB_TOKEN")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Failed to make http request %q: %v", url, err)
	}
	if user != "" && pass != "" {
		log.Printf("Fetching using GITHUB_USER and GITHUB_TOKEN")
		req.SetBasicAuth(user, pass)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Failed to fetch release info %q: %v", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("Error: %s", readBody(resp.Body))
		log.Fatalf("Bad status %d when fetching %q release info: %s", resp.StatusCode, url, resp.Status)
	}
	var release Release
	err = json.NewDecoder(resp.Body).Decode(&release)
	if err != nil {
		log.Fatalf("Failed to decode release info: %v", err)
	}
	err = resp.Body.Close()
	if err != nil {
		log.Fatalf("Failed to close body: %v", err)
	}

	for _, asset := range release.Assets {
		if matchName.MatchString(asset.Name) {
			return asset.BrowserDownloadURL, asset.Name
		}
	}
	log.Fatalf("Didn't find asset in info")
	return "", ""
}

// get a file for download
func getFile(url, fileName string) {
	log.Printf("Downloading %q from %q", fileName, url)

	out, err := os.Create(fileName)
	if err != nil {
		log.Fatalf("Failed to open %q: %v", fileName, err)
	}

	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Failed to fetch asset %q: %v", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("Error: %s", readBody(resp.Body))
		log.Fatalf("Bad status %d when fetching %q asset: %s", resp.StatusCode, url, resp.Status)
	}

	n, err := io.Copy(out, resp.Body)
	if err != nil {
		log.Fatalf("Error while downloading: %v", err)
	}

	err = resp.Body.Close()
	if err != nil {
		log.Fatalf("Failed to close body: %v", err)
	}
	err = out.Close()
	if err != nil {
		log.Fatalf("Failed to close output file: %v", err)
	}

	log.Printf("Downloaded %q (%d bytes)", fileName, n)
}

// run a shell command
func run(args ...string) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Fatalf("Failed to run %v: %v", args, err)
	}
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 2 {
		log.Fatalf("Syntax: %s <user/project> <name reg exp>", os.Args[0])
	}
	project, nameRe := args[0], args[1]
	if !matchProject.MatchString(project) {
		log.Fatalf("Project %q must be in form user/project", project)
	}
	matchName, err := regexp.Compile(nameRe)
	if err != nil {
		log.Fatalf("Invalid regexp for name %q: %v", nameRe, err)
	}

	assetURL, assetName := getAsset(project, matchName)
	fileName := filepath.Join(os.TempDir(), assetName)
	getFile(assetURL, fileName)

	if *install {
		log.Printf("Installing %s", fileName)
		run("sudo", "dpkg", "--force-bad-version", "-i", fileName)
		log.Printf("Installed %s", fileName)
	} else if *extract != "" {
		if *bindir == "" {
			log.Fatalf("Need to set -bindir")
		}
		log.Printf("Unpacking %s from %s and installing into %s", *extract, fileName, *bindir)
		run("tar", "xf", fileName, *extract)
		run("chmod", "a+x", *extract)
		run("mv", "-f", *extract, *bindir+"/")
	}
}
