// Copyright 2017 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shared

// This file exists to share path-related logic between both soong_ui and soong

import (
	"path/filepath"
)

// Joins the path strings in the argument list, taking absolute paths into
// account. That is, if one of the strings is an absolute path, the ones before
// are ignored.
func JoinPath(base string, rest ...string) string {
	result := base
	for _, next := range rest {
		if filepath.IsAbs(next) {
			result = next
		} else {
			result = filepath.Join(result, next)
		}
	}
	return result
}

// Given the out directory, returns the root of the temp directory (to be cleared at the start of each execution of Soong)
func TempDirForOutDir(outDir string) (tempPath string) {
	return filepath.Join(outDir, ".temp")
}
