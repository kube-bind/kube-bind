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

package web

import (
	"embed"
	"io/fs"
	"net/http"
)

// Static files embedded in the binary.
//
//go:embed dist/*
var StaticFiles embed.FS

// GetFileSystem returns the embedded file system for serving static files.
func GetFileSystem() http.FileSystem {
	// Create a sub-filesystem starting from dist
	subFS, err := fs.Sub(StaticFiles, "dist")
	if err != nil {
		panic("failed to create sub filesystem: " + err.Error())
	}
	return http.FS(subFS)
}
