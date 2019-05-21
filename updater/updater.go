// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

// TODO:
//  - move and copy ops should support find best using same logic in service.
// FUTURE:
//  - support restart replace on Windows

package main

import (
	"archive/zip"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/PaperCutSoftware/silver/lib/pathutils"
)

var (
	versionFile     = flag.String("f", ".version", "Set verison file")
	showVersion     = flag.Bool("v", false, "Display current installed version and exit")
	overrideVersion = flag.String("c", "", "Override current installed version")
	httpProxy       = flag.String("p", "", "Set HTTP proxy in format http://server:port")
	unsafeHTTP      = flag.Bool("unsafe", false, "Debug Only: Support non-https update checks for testing.")
)

const (
	profileFileName   string = "updater-profile.conf"
	keyIdentity       string = "identity"
	keyChannel        string = "channel"
	valChannelStable  string = "stable"
	valChannelBeta    string = "beta"
	valChannelExp     string = "experimental"
	customHeader      string = "X-profile-"
	idHeaderStr       string = customHeader + keyIdentity
	channelHeaderStr  string = customHeader + keyChannel
	timezoneHeaderStr string = customHeader + "timezone"
)

type UpgradeInfo struct {
	URL        string
	Version    string
	Sha1       string
	Sha256     string
	Operations []Operation
}

type Operation struct {
	Action string
	Args   []string
}

type Profile struct {
	Id      string `json:"id"`
	Channel string `json:"channel"`
}

func usage() {
	exeName := filepath.Base(os.Args[0])
	fmt.Fprintf(os.Stderr, "usage: %s [flags] [update url]\n", exeName)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "To generage or modify profile\n")
	fmt.Fprintf(os.Stderr, "  profile-set-random-id\n")
	fmt.Fprintf(os.Stderr, "\tGenerate a unique random id for this installation.\n")
	fmt.Fprintf(os.Stderr, "  profile-set-id <id-string>\n")
	fmt.Fprintf(os.Stderr, "\tUse the id-string as the unique identity.\n")
	fmt.Fprintf(os.Stderr, "  profile-set-channel <channel-string>\n")
	fmt.Fprintf(os.Stderr, "\tUse the channel-string as the distribution channel.\n")
	os.Exit(2)
}

func main() {

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "profile-set-random-id":
			if len(os.Args) == 2 {
				os.Exit(setRandomProfileId())
			} else {
				usage()
			}
		case "profile-set-id":
			if len(os.Args) == 3 {
				os.Exit(setProfileId(os.Args[2]))
			} else {
				usage()
			}
		case "profile-set-channel":
			if len(os.Args) == 3 {
				os.Exit(setProfileChannel(os.Args[2]))
			} else {
				usage()
			}
		}
	}

	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Printf("Current version: %s\n", readCurrentVersion())
		return
	}

	if flag.NArg() == 0 {
		usage()
	}
	checkURL := flag.Arg(0)

	if !*unsafeHTTP && !strings.HasPrefix(strings.ToLower(checkURL), "https") {
		fmt.Fprintf(os.Stderr, "ERROR: The update URL must be HTTPS for security reasons!\n")
		os.Exit(1)
	}

	setupHTTPProxy()
	ok, err := upgradeIfRequired(checkURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	if !ok {
		fmt.Println("No upgrade required")
	} else {
		fmt.Printf("Upgrade successful at %s.\n", time.Now().Format(time.RFC822))
	}
}

func upgradeIfRequired(checkURL string) (upgraded bool, err error) {
	currentVer := readCurrentVersion()
	if len(*overrideVersion) > 0 {
		currentVer = *overrideVersion
	}

	// Check update URL
	upgradeInfo, err := checkUpdate(checkURL, currentVer)
	if err != nil {
		// If we've got a proxy, have one more go with it off.
		if proxy := os.Getenv("HTTP_PROXY"); proxy != "" {
			fmt.Printf("Update check using proxy '%s' failed. Trying again without ...\n", proxy)
			turnOffHTTPProxy()
		}
		upgradeInfo, err = checkUpdate(checkURL, currentVer)
	}
	if err != nil {
		return false, err
	}

	if upgradeInfo == nil || upgradeInfo.URL == "" {
		// No upgrade required
		return false, nil
	}

	// Download
	fmt.Printf("Downloading version %s update from %s ...\n",
		upgradeInfo.Version,
		upgradeInfo.URL)

	zipfile, err := download(upgradeInfo.URL)
	if err != nil {
		return false, err
	}
	defer os.Remove(zipfile)

	if size, err := fileSize(zipfile); err == nil {
		fmt.Printf("Download complete (%d bytes).\n", size)
	}

	// Validate checksum
	var fileSum string
	var requiredSum string
	switch {
	case len(upgradeInfo.Sha256) > 0:
		requiredSum = upgradeInfo.Sha256
		fileSum = checksum("sha256", zipfile)
	case len(upgradeInfo.Sha1) > 0:
		requiredSum = upgradeInfo.Sha1
		fileSum = checksum("sha1", zipfile)
	default:
		return false, errors.New("Upgrade failed: The upgrade URL did not provide a checksum!")
	}

	if fileSum != requiredSum {
		return false, errors.New("Download checksum failed!")
	}

	// Unzip
	fmt.Println("Unzipping update ...")
	err = extractZip(zipfile, ".")
	if err != nil {
		return false, err
	}
	fmt.Println("Unzip complete.")

	// Perform any operations
	for _, op := range upgradeInfo.Operations {
		action := strings.ToLower(op.Action)
		var fn func([]string) error
		switch action {
		case "exec", "run":
			fn = execOp
		case "batchrename", "batch-rename":
			fn = batchRenameOp
		case "move", "mv":
			fn = moveOp
		case "copy", "cp":
			fn = copyOp
		case "remove", "rm", "del", "delete":
			fn = removeOp
		default:
			msg := fmt.Sprintf("Invalid operation action: '%s'", action)
			return false, errors.New(msg)
		}
		fmt.Printf("Performing operation '%s (%s)' ...\n",
			action, strings.Join(op.Args, ", "))
		if err := fn(op.Args); err != nil {
			msg := fmt.Sprintf("Operation failed with error: %v", err)
			return false, errors.New(msg)
		}
	}

	// Write version file
	ioutil.WriteFile(*versionFile, []byte(upgradeInfo.Version+"\n"), 0644)

	// Request service restart by writing the reload file into our root
	ioutil.WriteFile(".reload", []byte(""), 0644)

	// Success
	return true, nil
}

func getProfileFileName() (string, error) {
	// File containing the profile info should exist with the updater binary.
	updaterBin, err := os.Executable()
	if err != nil {
		return "", err
	}
	// Construct file name with absolute path.
	profileFile := filepath.Join(filepath.Dir(updaterBin), profileFileName)
	return profileFile, nil
}

func validateProfile(prf *Profile) error {
	isAlphaNumeric := regexp.MustCompile(`^[A-Za-z0-9]+$`).MatchString
	// Should validate the id string and channel.
	// For now lets assume id string is less than 256
	// characters and alphanumeric.
	// Channel to be alphanemeric and less than 10 characters.
	if !isAlphaNumeric(prf.Id) ||
		!isAlphaNumeric(prf.Channel) ||
		len(prf.Id) > 256 ||
		len(prf.Channel) > 10 {
		return errors.New("Profile Id or Channel format is invalid.")
	}
	return nil
}

func generateRandomIdString() (string, error) {
	nRand, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s", nRand), nil
}

func loadProfile(prf *Profile) (err error) {
	var fn string
	var data []byte
	if fn, err = getProfileFileName(); err != nil {
		return err
	}
	if data, err = ioutil.ReadFile(fn); err != nil {
		return err
	}
	if err = json.Unmarshal(data, prf); err != nil {
		return err
	}
	return validateProfile(prf)
}

func saveProfile(prf *Profile) (err error) {
	var fn string
	var data []byte
	if fn, err = getProfileFileName(); err != nil {
		return err
	}
	if data, err = json.Marshal(prf); err != nil {
		return err
	}
	return ioutil.WriteFile(fn, data, 0600)
}

func setRandomProfileId() int {
	strRand, err := generateRandomIdString()
	if err != nil {
		fmt.Errorf("Error: %v\n", err)
		return 1
	}
	prf := Profile{
		Id:      strRand,
		Channel: valChannelStable,
	}
	if err = saveProfile(&prf); err != nil {
		fmt.Errorf("Error: %v\n", err)
		return 1
	}
	return 0
}

func setProfileId(id string) int {
	prf := Profile{}
	err := loadProfile(&prf)
	prf.Id = id
	if err != nil {
		// Profile load failed. Doesn't exist or corrupted.
		// Set channel as well.
		prf.Channel = valChannelStable
	}
	if err = saveProfile(&prf); err != nil {
		fmt.Errorf("Error: %v.\n", err)
		return 1
	}
	return 0
}

func setProfileChannel(channel string) int {
	prf := Profile{}
	err := loadProfile(&prf)
	prf.Channel = channel
	if err != nil {
		// Profile load failed. Doesn't exist or corrupted.
		// Set id as well.
		strRand, errRand := generateRandomIdString()
		if errRand != nil {
			fmt.Errorf("Error: %v\n", errRand)
			return 1
		}
		prf.Id = strRand
	}
	if err = saveProfile(&prf); err != nil {
		fmt.Errorf("Error: %v.\n", err)
		return 1
	}
	return 0
}

func addIdProfileToRequestHeader(req *http.Request) {
	// Best effort to add identity profile to header.
	// Errors are logged allowing normal operation.

	// Add timezone. Gives a broad geo location.
	t := time.Now()
	zone, _ := t.Zone()
	req.Header.Set(timezoneHeaderStr, zone)

	// Add profile.
	prf := Profile{}
	if err := loadProfile(&prf); err != nil {
		fmt.Printf("Couldn't load profile: %v.\n", err)
		return
	}
	if len(prf.Id) > 0 {
		req.Header.Set(idHeaderStr, prf.Id)
	}
	if len(prf.Channel) > 0 {
		req.Header.Set(channelHeaderStr, prf.Channel)
	}
}

func fileSize(file string) (size int64, err error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

func checkUpdate(url string, currentVer string) (*UpgradeInfo, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url+"?version="+currentVer, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Update Check")
	addIdProfileToRequestHeader(req)

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotModified {
		return nil, nil
	}

	if res.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("Got an error from the update url: %s", res.Status)
	}

	dec := json.NewDecoder(res.Body)
	var info UpgradeInfo
	err = dec.Decode(&info)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to parse JSON at %s : %v", url, err))
	}

	if info.Version != "" && info.Version == currentVer {
		// Same version!
		return nil, nil
	}

	return &info, nil
}

func download(url string) (string, error) {
	outfile, err := ioutil.TempFile("", "update-")
	if err != nil {
		return "", err
	}

	resp, err := http.Get(url)
	if err != nil {
		outfile.Close()
		os.Remove(outfile.Name())
		return "", err
	}
	defer resp.Body.Close()

	_, err = io.Copy(outfile, resp.Body)
	if err != nil {
		outfile.Close()
		os.Remove(outfile.Name())
		return "", err
	}
	outfile.Close()
	return outfile.Name(), nil
}

func checksum(hashType string, file string) string {
	var hasher hash.Hash

	switch {
	case hashType == "sha256":
		hasher = sha256.New()
	case hashType == "sha1":
		hasher = sha1.New()
	default:
		hasher = sha1.New()
	}
	f, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	io.Copy(hasher, f)
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

func extractZip(zipfile, dest string) error {
	r, err := zip.OpenReader(zipfile)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if err := extractZipItem(f, dest); err != nil {
			return err
		}
	}
	return nil
}

func extractZipItem(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	path := filepath.Join(dest, f.Name)
	if f.FileInfo().IsDir() {
		os.MkdirAll(path, f.Mode())
	} else {
		f, err := os.OpenFile(
			path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(f, rc)
		if err != nil {
			return err
		}
	}
	return nil
}

func readCurrentVersion() string {
	ver := "1"
	if dat, err := ioutil.ReadFile(*versionFile); err == nil {
		ver = strings.TrimSpace(string(dat))
	}
	return ver
}

func setupHTTPProxy() {
	// Force if set via flag
	if len(*httpProxy) > 0 {
		os.Setenv("HTTP_PROXY", *httpProxy)
		return
	}
	// Check Silver Environment
	proxy := os.Getenv("SILVER_HTTP_PROXY")
	if proxy != "" {
		os.Setenv("HTTP_PROXY", proxy)
		return
	}
	// Proxy conf file
	if dat, err := ioutil.ReadFile("http-proxy.conf"); err == nil {
		proxy = strings.TrimSpace(string(dat))
	}
	if proxy != "" {
		os.Setenv("HTTP_PROXY", proxy)
		return
	}
}

func turnOffHTTPProxy() {
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t.Proxy = func(req *http.Request) (*url.URL, error) {
			return nil, nil
		}
	}

}

func execOp(args []string) (err error) {
	if len(args) < 1 {
		return errors.New("Invalid exec operation format - arg expected.")
	}
	cmd := args[0]
	fmt.Printf("Running install command: %s\n", strings.Join(args, " "))
	os.Chmod(cmd, 0755)
	c := exec.Command(cmd, args[1:]...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	err = c.Run()
	return err
}

func batchRenameOp(args []string) error {
	if len(args) != 3 {
		return errors.New("Invalid rename operation format - three args expected.")
	}
	root := args[0]
	find := args[1]
	replacement := args[2]
	fmt.Printf("Running batch rename operation on root %s ('%s' => '%s')\n", root, find, replacement)
	return batchRename(root, find, replacement)
}

func batchRename(root, find, replacement string) error {
	matches, err := filepath.Glob(root)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return nil
	}

	re, err := regexp.Compile(find)
	if err != nil {
		return err
	}

	renameCnt := 0
	visitFn := func(path string, fi os.FileInfo, errin error) error {
		name := fi.Name()
		newName := re.ReplaceAllString(name, replacement)
		if name == newName {
			return nil
		}
		if !fileExists(path) {
			return nil
		}

		newPath := filepath.Join(filepath.Dir(path), newName)
		fmt.Printf("Renaming '%s' to '%s' ...\n", path, newPath)
		err := os.Rename(path, newPath)
		if err != nil {
			return err
		}
		renameCnt++
		return nil
	}

	for _, match := range matches {
		err := filepath.Walk(match, visitFn)
		if err != nil {
			return err
		}
	}
	fmt.Printf("Renamed %d files.\n", renameCnt)
	return nil
}

func moveOp(args []string) error {
	if len(args) != 2 {
		return errors.New("Invalid copy operation format - two args expected.")
	}
	src := pathutils.FindLastFile(args[0])
	fmt.Printf("Moving '%s' to '%s'...\n", src, args[1])
	return os.Rename(src, args[1])
}

func removeOp(args []string) error {
	if len(args) != 1 {
		return errors.New("Invalid remove operation format - one arg file expected.")
	}
	path := args[0]
	matches, err := filepath.Glob(path)
	if err != nil {
		return err
	}
	removeCnt := 0
	for _, match := range matches {
		fmt.Printf("Removing '%s' ...\n", match)
		if err := os.RemoveAll(match); err != nil {
			// Don't exit... best effort
			fmt.Printf("Problem removing %s: %v\n", match, err)
		}
		removeCnt++
	}
	fmt.Printf("Removed %d files.\n", removeCnt)
	return nil
}

func copyOp(args []string) error {
	if len(args) != 2 {
		return errors.New("Invalid copy operation format - two args expected.")
	}
	src := pathutils.FindLastFile(args[0])
	fmt.Printf("Copying '%s' to '%s'...\n", src, args[1])
	return copyFile(src, args[1])
}

func copyFile(src, dest string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := io.Copy(d, s); err != nil {
		d.Close()
		return err
	}
	return d.Close()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
