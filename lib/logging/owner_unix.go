// +build !windows

package logging

import (
	"os"
	"os/user"
	"strconv"
)

func changeOwnerOfFile(name string, owner string) error {
	// If owner is defined, change the owner of the log file to this user
	if owner != "" {
		ownerUser, err := user.Lookup(owner)
		if err != nil {
			return err
		}
		uid, err := strconv.Atoi(ownerUser.Uid)
		if err != nil {
			return err
		}
		gid, err := strconv.Atoi(ownerUser.Gid)
		if err != nil {
			return err
		}
		return os.Chown(name, uid, gid)
	}
	return nil
}
