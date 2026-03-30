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
	"context"
	"os/exec"
	"runtime"
)

// openBrowser opens the given URL in the default browser
func OpenBrowser(url string) error {
	var cmd *exec.Cmd

	// Determine the command based on the operating system
	switch runtime.GOOS {
	case "windows":
		cmd = exec.CommandContext(context.Background(), "cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.CommandContext(context.Background(), "open", url)
	default: // Linux and other Unix-like systems
		cmd = exec.CommandContext(context.Background(), "xdg-open", url)
	}

	return cmd.Run()
}
