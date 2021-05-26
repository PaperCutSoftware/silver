// Copyright (c) 2021 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
package svcutil

import (
	"errors"
	"fmt"
	"syscall"
	"time"

	"golang.org/x/sys/windows/svc/mgr"
)

const (
	MaxDelay                     = syscall.INFINITE * time.Millisecond
	InfiniteFailCountResetPeriod = syscall.INFINITE * time.Second
)

// SetServiceToRestart sets a service to automatically restart on failure after waiting for a duration specified, and
// the time duration after which to reset the service failure count to zero if there are no failures. The specified
// service has to be already registered in the SCM. If the restart delay is negative or greater than MaxDelay, or the
// time duraion to reset fail count is negative, an error is returned. If the time duration to rest fail count is
// greater than or equal InfiniteFailCountResetPeriod, the service failure count is never reset.
func SetServiceToRestart(conf RestartConfig) error {
	switch {
	case conf.RestartDelay < 0:
		return errors.New("Invalid delay time")
	case conf.RestartDelay > MaxDelay:
		return errors.New("Exceeding maximum delay time")
	case conf.ResetFailCountAfter < 0:
		return errors.New("Invalid reset period")
	case conf.ResetFailCountAfter > InfiniteFailCountResetPeriod:
		conf.ResetFailCountAfter = InfiniteFailCountResetPeriod
	}

	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("Failed to open service control manager: %v", err)
	}
	defer func() {
		_ = manager.Disconnect()
	}()

	service, err := manager.OpenService(conf.ServiceName)
	if err != nil {
		return fmt.Errorf("Failed to open service %s: %v", conf.ServiceName, err)
	}
	defer func() {
		_ = service.Close()
	}()

	actions := []mgr.RecoveryAction{{
		Type:  mgr.ServiceRestart,
		Delay: conf.RestartDelay,
	}}

	if err = service.SetRecoveryActions(actions, uint32(conf.ResetFailCountAfter/time.Second)); err != nil {
		return fmt.Errorf("Failed to set recovery action: %v", err)
	}
	return nil
}
