// Copyright (C) 2023 Andrew Ayer
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of this software and associated documentation files (the "Software"),
// to deal in the Software without restriction, including without limitation
// the rights to use, copy, modify, merge, publish, distribute, sublicense,
// and/or sell copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
// THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR
// OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
// ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
// OTHER DEALINGS IN THE SOFTWARE.
//
// Except as contained in this notice, the name(s) of the above copyright
// holders shall not be used in advertising or otherwise to promote the
// sale, use or other dealings in this Software without prior written
// authorization.

package depproxy

import (
	"bufio"
	"fmt"
	"io"
	"path"
	"strings"

	"src.agwa.name/depproxy/internal/goproxy"
)

type AllowedModule struct {
	// Exactly one of Path and PathPattern are set
	Path        goproxy.ModulePath
	PathPattern string                // if set, is a valid path.Pattern
	Version     goproxy.ModuleVersion // if not set, all versions are allowed
}

func (module *AllowedModule) matchesPath(modulePath goproxy.ModulePath) bool {
	if module.Path.IsSet() {
		return module.Path == modulePath
	} else {
		matched, _ := path.Match(module.PathPattern, modulePath.String())
		return matched
	}
}

func (m *AllowedModule) matchesVersion(version goproxy.ModuleVersion) bool {
	return m.Version.IsEmpty() || m.Version == version
}

func (m *AllowedModule) matches(path goproxy.ModulePath, version goproxy.ModuleVersion) bool {
	return m.matchesPath(path) && m.matchesVersion(version)
}

func ReadAllowedModules(r io.Reader) ([]AllowedModule, error) {
	modules := []AllowedModule{}

	scanner := bufio.NewScanner(r)
	lineno := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineno++

		if strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		} else if len(f) != 2 {
			return nil, fmt.Errorf("syntax error on line %d: two fields expected, but %d provided", lineno, len(f))
		}

		var module AllowedModule
		if strings.Contains(f[0], "*") {
			if _, err := path.Match(f[0], ""); err != nil {
				return nil, fmt.Errorf("syntax error on line %d: module path pattern is invalid", lineno)
			}
			module.PathPattern = f[0]
		} else {
			modulePath, err := goproxy.MakeModulePath(f[0])
			if err != nil {
				return nil, fmt.Errorf("syntax error on line %d: %w", lineno, err)
			}
			module.Path = modulePath
		}

		if f[1] != "*" {
			moduleVersion, err := goproxy.MakeModuleVersion(f[1])
			if err != nil {
				return nil, fmt.Errorf("syntax error on line %d: %w", lineno, err)
			}
			module.Version = moduleVersion
		}

		if module.PathPattern != "" && module.Version.IsSet() {
			return nil, fmt.Errorf("error on line %d: version must be '*' when a path pattern is used", lineno)
		}

		modules = append(modules, module)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return modules, nil
}
