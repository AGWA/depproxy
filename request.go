package depproxy

import (
	"fmt"
	"strings"

	"src.agwa.name/depproxy/goproxy"
)

type proxyRequest interface {
	path() string
}

type latestRequest struct{}
type listRequest struct{}
type infoRequest struct {
	Version goproxy.ModuleVersion
}
type modRequest struct {
	Version goproxy.ModuleVersion
}
type zipRequest struct {
	Version goproxy.ModuleVersion
}

func (latestRequest) path() string { return "@latest" }
func (listRequest) path() string   { return "@v/list" }
func (r infoRequest) path() string { return "@v/" + r.Version.Escaped() + ".info" }
func (r modRequest) path() string  { return "@v/" + r.Version.Escaped() + ".mod" }
func (r zipRequest) path() string  { return "@v/" + r.Version.Escaped() + ".zip" }

func parseProxyPath(path string) (goproxy.ModulePath, proxyRequest, error) {
	if strings.HasSuffix(path, "/@latest") {
		if modulePath, err := goproxy.UnescapeModulePath(strings.TrimSuffix(path, "/@latest")); err == nil {
			return modulePath, latestRequest{}, nil
		} else {
			return "", nil, fmt.Errorf("invalid module path: %w", err)
		}
	} else if strings.HasSuffix(path, "/@v/list") {
		if modulePath, err := goproxy.UnescapeModulePath(strings.TrimSuffix(path, "/@v/list")); err == nil {
			return modulePath, listRequest{}, nil
		} else {
			return "", nil, fmt.Errorf("invalid module path: %w", err)
		}
	} else if i := strings.LastIndex(path, "/@v/"); i != -1 {
		modulePath, err := goproxy.UnescapeModulePath(path[:i])
		if err != nil {
			return "", nil, fmt.Errorf("invalid module path: %w", err)
		}
		filename := path[i+4:]
		if strings.HasSuffix(filename, ".info") {
			if version, err := goproxy.UnescapeModuleVersion(strings.TrimSuffix(filename, ".info")); err == nil {
				return modulePath, infoRequest{Version: version}, nil
			} else {
				return "", nil, fmt.Errorf("invalid module version: %w", err)
			}
		} else if strings.HasSuffix(filename, ".mod") {
			if version, err := goproxy.UnescapeModuleVersion(strings.TrimSuffix(filename, ".mod")); err == nil {
				return modulePath, modRequest{Version: version}, nil
			} else {
				return "", nil, fmt.Errorf("invalid module version: %w", err)
			}
		} else if strings.HasSuffix(filename, ".zip") {
			if version, err := goproxy.UnescapeModuleVersion(strings.TrimSuffix(filename, ".zip")); err == nil {
				return modulePath, zipRequest{Version: version}, nil
			} else {
				return "", nil, fmt.Errorf("invalid module version: %w", err)
			}
		}
	}
	return "", nil, fmt.Errorf("unrecognized proxy request")
}
