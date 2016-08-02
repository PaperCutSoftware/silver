// Copyright (c) 2016 PaperCut Software International Pty. Ltd.

package osutils

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

func getHTTPProxy() (string, error) {
	out, err := exec.Command("system_profiler", "SPNetworkDataType").Output()
	if err != nil {
		return "", fmt.Errorf("Unable to run system_profiler: %v", err)
	}
	sysData := string(out)
	/*
			 Format:
			 HTTP Proxy Enabled: Yes
		     HTTP Proxy Port: 8888
		     HTTP Proxy Server: localhost
	*/
	if strings.Contains(sysData, "HTTP Proxy Enabled: Yes") {
		// FIXME: If more than one match, pick?
		server := regexp.MustCompile(`HTTP Proxy Server:\s+(\w+)`).FindStringSubmatch(sysData)
		port := regexp.MustCompile(`HTTP Proxy Port:\s+(\d+)`).FindStringSubmatch(sysData)
		if len(server) > 1 && len(port) > 1 {
			return fmt.Sprintf("%s:%s", server[1], port[1]), nil
		}
	}
	return "", nil
}
