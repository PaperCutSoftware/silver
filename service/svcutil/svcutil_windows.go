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
	MaxDelay            = syscall.INFINITE * time.Millisecond
	InfiniteResetPeriod = syscall.INFINITE * time.Second
)

// SetServiceToRestart sets the service named name to automatically restart after waiting for a duration specified by
// delay when the service fails, and the time after which to reset the service failure count to zero if there are no
// failures. The service that specifies has to already be registered in the SCM. If delay is negative or greater than
// MaxDelay, or resetPeriod is negative, an error is returned. If resetPeriod is grater than or equal
// InfiniteResetPeriod, the service failure count is never reset.
func SetServiceToRestart(conf RestartConfig) error {
	switch {
	case conf.RestartDelay < 0:
		return errors.New("Invalid delay time")
	case conf.RestartDelay > MaxDelay:
		return errors.New("Exceeding maximum delay time")
	case conf.ResetPeriod < 0:
		return errors.New("Invalid reset period")
	case conf.ResetPeriod > InfiniteResetPeriod:
		conf.ResetPeriod = InfiniteResetPeriod
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

	if err = service.SetRecoveryActions(actions, uint32(conf.ResetPeriod/time.Second)); err != nil {
		return fmt.Errorf("Failed to set recovery action: %v", err)
	}
	return nil
}
