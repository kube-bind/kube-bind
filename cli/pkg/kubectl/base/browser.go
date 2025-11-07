/*
Copyright 2025 The Kube Bind Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package base

import (
	"os/exec"
	"strings"
)

// openBrowser opens the given URL in the default browser
func OpenBrowser(url string) error {
	var cmd *exec.Cmd

	// Determine the command based on the operating system
	switch {
	case isWindows():
		cmd = exec.Command("cmd", "/c", "start", url)
	case isMacOS():
		cmd = exec.Command("open", url)
	default: // Linux and other Unix-like systems
		cmd = exec.Command("xdg-open", url)
	}

	return cmd.Run()
}

// isWindows checks if the current OS is Windows
func isWindows() bool {
	return strings.Contains(strings.ToLower(exec.Command("uname").String()), "windows")
}

// isMacOS checks if the current OS is macOS
func isMacOS() bool {
	cmd := exec.Command("uname")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "Darwin"
}
