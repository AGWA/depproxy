package depproxy

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type AllowedModule struct {
	PathPattern    string
	VersionPattern string
}

func (m *AllowedModule) matchesPath(path string) (bool, error) {
	matched, err := filepath.Match(m.PathPattern, path)
	if err != nil {
		return false, fmt.Errorf("path pattern %q is malformed", m.PathPattern)
	}
	return matched, nil
}

func (m *AllowedModule) matches(path, version string) (bool, error) {
	if matched, err := m.matchesPath(path); err != nil {
		return false, err
	} else if !matched {
		return false, nil
	}
	if matched, err := filepath.Match(m.VersionPattern, version); err != nil {
		return false, fmt.Errorf("version pattern %q is malformed", m.VersionPattern)
	} else if !matched {
		return false, nil
	}
	return true, nil
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
		modules = append(modules, AllowedModule{
			PathPattern:    f[0],
			VersionPattern: f[1],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return modules, nil
}
