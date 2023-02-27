package goproxy

import (
	"fmt"
	"strings"
)

type Request interface {
	Path() string
}

type LatestRequest struct{}
type ListRequest struct{}
type InfoRequest struct {
	Version ModuleVersion
}
type ModRequest struct {
	Version ModuleVersion
}
type ZipRequest struct {
	Version ModuleVersion
}

func (LatestRequest) Path() string { return "@latest" }
func (ListRequest) Path() string   { return "@v/list" }
func (r InfoRequest) Path() string { return "@v/" + r.Version.Escaped() + ".info" }
func (r ModRequest) Path() string  { return "@v/" + r.Version.Escaped() + ".mod" }
func (r ZipRequest) Path() string  { return "@v/" + r.Version.Escaped() + ".zip" }

func ParseRequestPath(path string) (ModulePath, Request, error) {
	if strings.HasSuffix(path, "/@latest") {
		if modulePath, err := UnescapeModulePath(strings.TrimSuffix(path, "/@latest")); err == nil {
			return modulePath, LatestRequest{}, nil
		} else {
			return "", nil, fmt.Errorf("invalid module path: %w", err)
		}
	} else if strings.HasSuffix(path, "/@v/list") {
		if modulePath, err := UnescapeModulePath(strings.TrimSuffix(path, "/@v/list")); err == nil {
			return modulePath, ListRequest{}, nil
		} else {
			return "", nil, fmt.Errorf("invalid module path: %w", err)
		}
	} else if i := strings.LastIndex(path, "/@v/"); i != -1 {
		modulePath, err := UnescapeModulePath(path[:i])
		if err != nil {
			return "", nil, fmt.Errorf("invalid module path: %w", err)
		}
		filename := path[i+4:]
		if strings.HasSuffix(filename, ".info") {
			if version, err := UnescapeModuleVersion(strings.TrimSuffix(filename, ".info")); err == nil {
				return modulePath, InfoRequest{Version: version}, nil
			} else {
				return "", nil, fmt.Errorf("invalid module version: %w", err)
			}
		} else if strings.HasSuffix(filename, ".mod") {
			if version, err := UnescapeModuleVersion(strings.TrimSuffix(filename, ".mod")); err == nil {
				return modulePath, ModRequest{Version: version}, nil
			} else {
				return "", nil, fmt.Errorf("invalid module version: %w", err)
			}
		} else if strings.HasSuffix(filename, ".zip") {
			if version, err := UnescapeModuleVersion(strings.TrimSuffix(filename, ".zip")); err == nil {
				return modulePath, ZipRequest{Version: version}, nil
			} else {
				return "", nil, fmt.Errorf("invalid module version: %w", err)
			}
		}
	}
	return "", nil, fmt.Errorf("unrecognized proxy request")
}
