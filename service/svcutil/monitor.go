package svcutil

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type MonitorConfig struct {
	URL                   string
	StartupDelay          time.Duration
	Interval              time.Duration
	Timeout               time.Duration
	RestartOnFailureCount int
}

type serviceMonitor struct {
	config      MonitorConfig
	logger      *log.Logger
	serviceName string
}

func (sm *serviceMonitor) start(terminate chan struct{}) chan struct{} {
	monitor := make(chan struct{})
	go func() {
		time.Sleep(sm.config.StartupDelay)
		failureCount := 0
		isTerminate:
		for {
			select {
			case <-time.After(sm.config.Interval):
			case <-terminate:
				break isTerminate
			}
			ok, err := pingURL(sm.config.URL, sm.config.Timeout)
			if err != nil {
				sm.logger.Printf("%s: Monitor ping error '%v'", sm.serviceName, err)
				failureCount = 0
			} else if !ok {
				failureCount++
				sm.logger.Printf("%s: Monitor detected error - '%v'", sm.serviceName, err)
			}
			if failureCount > sm.config.RestartOnFailureCount {
				sm.logger.Printf("%s: Service not responding. Forcing shutdown. (failures: %d)",
					sm.serviceName, failureCount)
				break isTerminate
			}

		}
		close(monitor)
	}()
	return monitor
}

var pingFileCache = struct {
	sync.Mutex
	m map[string]string
}{m: make(map[string]string)}

func pingURL(pingURL string, timeout time.Duration) (ok bool, err error) {
	u, err := url.Parse(pingURL)
	if err != nil {
		return true, errors.New("Invalid Ping URL!") // Assume OK
	}
	switch strings.ToLower(u.Scheme) {
	case "tcp":
		return pingTCP(u.Host, timeout)
	case "echo":
		return pingTCPEcho(u.Host, timeout)
	case "http":
		fallthrough
	case "https":
		return pingHTTP(pingURL, timeout)
	case "file":
		return pingFile(pingURL)
	default:
		return true, errors.New("Unsupported URL Scheme") // Assume OK
	}
}

func pingTCP(host string, timeout time.Duration) (ok bool, err error) {
	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		return false, err
	}
	conn.Close()
	return true, nil
}

func pingTCPEcho(host string, timeout time.Duration) (ok bool, err error) {
	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))

	// Challenge the server with a unique ping
	ping := fmt.Sprintf("ping-%d", time.Now().UTC())
	if _, err := fmt.Fprintf(conn, ping); err != nil {
		return false, err
	}
	buf := make([]byte, 1024)
	if _, err := conn.Read(buf); err != nil {
		return false, err
	}
	if !strings.Contains(string(buf), ping) {
		return false, errors.New("Server did not echo")
	}
	return true, nil
}

func pingFile(fileURL string) (ok bool, err error) {
	file := strings.TrimPrefix(fileURL, "file://")
	info, err := os.Stat(file)
	if err != nil {
		return true, err
	}
	stamp := fmt.Sprintf("%d%d", info.Size(), info.ModTime().UnixNano())
	pingFileCache.Lock()
	defer pingFileCache.Unlock()
	if v, ok := pingFileCache.m[file]; ok {
		if v == stamp {
			// No change!
			return false, errors.New(fmt.Sprintf("File %s did not change", file))
		}
	}
	pingFileCache.m[file] = stamp
	return true, nil
}
