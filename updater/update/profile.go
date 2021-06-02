/*
 * Copyright © 2021 PaperCut Software International Pty. Ltd.
 */

package update

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const (
	profileFileName          string = "updater-profile.conf"
	valChannelStable         string = "stable"
	valChannelBeta           string = "beta"
	valChannelExp            string = "experimental"
	headerProfileIDKey       string = "X-profile-identity"
	headerProfileChannelKey  string = "X-profile-channel"
	headerProfileTimezoneKey string = "X-profile-timezone"
)

type Profile struct {
	Id      string `json:"id"`
	Channel string `json:"channel"`
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

func SetRandomProfileID() int {
	strRand, err := generateRandomIDString()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return SetProfileID(strRand)
}

func SetProfileID(id string) int {
	prf := Profile{}
	err := loadProfile(&prf)
	prf.Id = id
	if err != nil {
		// Profile load failed. Doesn't exist or corrupted.
		// Set channel as well.
		prf.Channel = valChannelStable
	}
	if err = saveProfile(&prf); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v.\n", err)
		return 1
	}
	return 0
}

func SetProfileChannel(channel string) int {
	prf := Profile{}
	err := loadProfile(&prf)
	prf.Channel = channel
	if err != nil {
		// Profile load failed. Doesn't exist or corrupted.
		// Set id as well.
		strRand, errRand := generateRandomIDString()
		if errRand != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", errRand)
			return 1
		}
		prf.Id = strRand
	}
	if err = saveProfile(&prf); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v.\n", err)
		return 1
	}
	return 0
}

func addIDProfileToRequestHeader(req *http.Request) {
	// Best effort to add identity profile to header.
	// Errors are logged allowing normal operation.

	// Add timezone. Gives a broad geo location.
	t := time.Now()
	zone, _ := t.Zone()
	req.Header.Set(headerProfileTimezoneKey, zone)

	// Add profile.
	prf := Profile{}
	if err := loadProfile(&prf); err != nil {
		fmt.Printf("Couldn't load profile: %v.\n", err)
		return
	}
	if len(prf.Id) > 0 {
		fmt.Printf("Using Id: %v.\n", prf.Id)
		req.Header.Set(headerProfileIDKey, prf.Id)
	}
	if len(prf.Channel) > 0 {
		fmt.Printf("Using Channel: %v.\n", prf.Channel)
		req.Header.Set(headerProfileChannelKey, prf.Channel)
	}
}

func generateRandomIDString() (string, error) {
	nRand, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s", nRand), nil
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
	isAlphaNumeric := regexp.MustCompile(`^[A-Za-z0-9-|]+$`).MatchString
	// Should validate the id string and channel.
	// For now lets assume id string is less than 256
	// characters and alphanumeric.
	// Channel to be alphanumeric and less than 10 characters.
	if !isAlphaNumeric(prf.Id) ||
		!isAlphaNumeric(prf.Channel) ||
		len(prf.Id) > 256 ||
		len(prf.Channel) > 10 {
		return errors.New("Profile Id or Channel format is invalid.")
	}
	return nil
}
