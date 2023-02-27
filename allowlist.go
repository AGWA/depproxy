package depproxy

import (
	"bufio"
	"fmt"
	"io"
	"path"
	"strings"

	"src.agwa.name/depproxy/goproxy"
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
