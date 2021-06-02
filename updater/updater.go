// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2014-2021 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

// TODO:
//  - move and copy ops should support find best using same logic in service.
// FUTURE:
//  - support restart replace on Windows

package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/papercutsoftware/silver/service/config"
	"github.com/papercutsoftware/silver/updater/update"
)

func upgradeIfRequired(checkURL string) (upgraded bool, err error) {
	// Check update URL
	upgradeInfo, err := checkUpdate(detectCurrentVersion(), checkURL)
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
	defer func() {
		if err := os.Remove(zipfile); err != nil {
			fmt.Printf("Failed to remove ZIP %s : %v\n", zipfile, err)
		}
	}()

	if size, err := fileSize(zipfile); err == nil {
		fmt.Printf("Download complete (%d bytes).\n", size)
	}

	// Validate checksum
	err = update.ValidateCheckSum(upgradeInfo, zipfile)
	if err != nil {
		return false, err
	}

	// Unzip
	fmt.Println("Unzipping update ...")
	err = update.ExtractZip(zipfile, ".")
	if err != nil {
		return false, err
	}
	fmt.Println("Unzip complete.")

	// Perform any operations
	err = update.RunUpgradeOps(upgradeInfo)
	if err != nil {
		return false, err
	}

	upgradeComplete(upgradeInfo)

	// Success
	return true, nil
}

func detectCurrentVersion() string {
	currentVer := update.ReadCurrentVersion(*versionFile)
	if len(*overrideVersion) > 0 {
		currentVer = *overrideVersion
	}
	return currentVer
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

func checkUpdate(currentVer, checkURL string) (*update.UpgradeInfo, error) {
	upgradeInfo, err := update.Check(checkURL, currentVer)
	if err != nil {
		// If we've got a proxy, have one more go with it off.
		if proxy := os.Getenv("HTTP_PROXY"); proxy != "" {
			fmt.Printf("Update check using proxy '%s' failed. Trying again without ...\n", proxy)
			turnOffHTTPProxy()
		}
		upgradeInfo, err = update.Check(checkURL, currentVer)
	}
	return upgradeInfo, err
}

func upgradeComplete(upgradeInfo *update.UpgradeInfo) {
	// Write version file
	_ = ioutil.WriteFile(*versionFile, []byte(upgradeInfo.Version+"\n"), 0644)

	// Request service restart by writing the reload file into our root
	_ = ioutil.WriteFile(config.ReloadFileName, []byte(""), 0644)
}

func download(url string) (string, error) {
	outfile, err := ioutil.TempFile("", "update-")
	if err != nil {
		return "", err
	}

	resp, err := http.Get(url)
	if err != nil {
		_ = outfile.Close()
		_ = os.Remove(outfile.Name())
		return "", err
	}
	defer resp.Body.Close()

	_, err = io.Copy(outfile, resp.Body)
	if err != nil {
		_ = outfile.Close()
		_ = os.Remove(outfile.Name())
		return "", err
	}
	_ = outfile.Close()
	return outfile.Name(), nil
}
