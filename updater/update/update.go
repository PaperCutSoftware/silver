// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2014-2021 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/papercutsoftware/silver/lib/jsonsig"
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

func Check(updateURL string, currentVer string, publicKey string) (*UpgradeInfo, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", updateURL+"?version="+url.QueryEscape(currentVer), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Update Check")
	addIDProfileToRequestHeader(req)

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotModified {
		return nil, nil
	}

	if res.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("Got an error from the update url: %d (%s) ", res.StatusCode, res.Status)
	}

	// Limit the read to 1MB to prevent unbounded memory usage from a malicious server.
	rawPayload, err := io.ReadAll(io.LimitReader(res.Body, 1*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read update response body: %w", err)
	}

	// If a public key is provided, verify the signature of the manifest.
	if publicKey != "" {
		valid, err := jsonsig.Verify(rawPayload, publicKey)
		if err != nil {
			return nil, fmt.Errorf("error verifying update manifest signature: %w", err)
		}
		if !valid {
			return nil, errors.New("update manifest signature is invalid")
		}
	}

	var info UpgradeInfo
	err = json.Unmarshal(rawPayload, &info)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse JSON at %s : %v", updateURL, err)
	}

	if info.Version != "" && info.Version == currentVer {
		// Same version!
		return nil, nil
	}

	return &info, nil
}

func ValidateCheckSum(upgradeInfo *UpgradeInfo, zipfile string) error {
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
		return errors.New("Upgrade failed: The upgrade URL did not provide a checksum!")
	}

	if fileSum != requiredSum {
		return errors.New("Download checksum failed!")
	}
	return nil
}

func RunUpgradeOps(upgradeInfo *UpgradeInfo) error {
	for _, op := range upgradeInfo.Operations {
		action := strings.ToLower(op.Action)
		var fn func([]string) error
		switch action {
		case "exec", "run":
			fn = ExecOp
		case "batchrename", "batch-rename":
			fn = BatchRenameOp
		case "move", "mv":
			fn = MoveOp
		case "copy", "cp":
			fn = CopyOp
		case "remove", "rm", "del", "delete":
			fn = RemoveOp
		default:
			msg := fmt.Sprintf("Invalid operation action: %q", action)
			return errors.New(msg)
		}
		fmt.Printf("Performing operation '%s (%s)' ...\n",
			action, strings.Join(op.Args, ", "))
		if err := fn(op.Args); err != nil {
			msg := fmt.Sprintf("Operation failed with error: %v", err)
			return errors.New(msg)
		}
	}
	return nil
}
