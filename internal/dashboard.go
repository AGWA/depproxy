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
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"

	"golang.org/x/sync/errgroup"
	"src.agwa.name/depproxy/internal/goproxy"
)

var dashboardTemplate = template.Must(template.ParseFS(content, "templates/dashboard.html"))

type allowedModuleInfo struct {
	AllowedModule
	CurrentInfo *goproxy.ModuleInfo
	CurrentErr  error
	LatestInfo  *goproxy.ModuleInfo
	LatestErr   error
}

func (mod *allowedModuleInfo) OutOfDate() bool {
	return mod.CurrentInfo != nil && mod.LatestInfo != nil && mod.CurrentInfo.Version.Compare(mod.LatestInfo.Version) == -1
}

func (mod *allowedModuleInfo) VCSDiff() string {
	if mod.CurrentInfo != nil && mod.LatestInfo != nil &&
		mod.CurrentInfo.Origin != nil && mod.LatestInfo.Origin != nil &&
		mod.CurrentInfo.Origin.VCS == mod.LatestInfo.Origin.VCS &&
		mod.CurrentInfo.Origin.URL == mod.LatestInfo.Origin.URL {

		return vcsDiff(mod.CurrentInfo.Origin.VCS, mod.CurrentInfo.Origin.URL, mod.CurrentInfo.Origin, mod.LatestInfo.Origin)
	} else {
		return ""
	}
}

func processModuleInfoResponse(resp *http.Response, err error) (*goproxy.ModuleInfo, error) {
	if err != nil {
		return nil, fmt.Errorf("error communicating with upstream proxy: %w", err)
	}
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("error communicating with upstream proxy: %w", err)
	}

	info := new(goproxy.ModuleInfo)
	if err := json.Unmarshal(respBody, info); err != nil {
		return nil, fmt.Errorf("error parsing info JSON received from upstream proxy: %w", err)
	}

	return info, nil
}

func (s *Server) getModuleInfo(ctx context.Context, module goproxy.ModulePath, version goproxy.ModuleVersion) (*goproxy.ModuleInfo, error) {
	return processModuleInfoResponse(s.requestUpstream(ctx, module, goproxy.InfoRequest{Version: version}))
}

func (s *Server) getLatestModuleInfo(ctx context.Context, module goproxy.ModulePath) (*goproxy.ModuleInfo, error) {
	return processModuleInfoResponse(s.requestUpstream(ctx, module, goproxy.LatestRequest{}))
}

func (s *Server) getAllowedModulesInfo(ctx context.Context) ([]allowedModuleInfo, error) {
	modules := make([]allowedModuleInfo, len(s.AllowedModules))
	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(11)
	group.Go(func() error {
		for i := range s.AllowedModules {
			i := i
			if ctx.Err() != nil {
				return ctx.Err()
			}
			modules[i].AllowedModule = s.AllowedModules[i]
			if modules[i].Path.IsEmpty() {
				continue
			}
			if modules[i].Version.IsSet() {
				group.Go(func() error {
					modules[i].CurrentInfo, modules[i].CurrentErr = s.getModuleInfo(ctx, modules[i].Path, modules[i].Version)
					return nil
				})
			}
			group.Go(func() error {
				modules[i].LatestInfo, modules[i].LatestErr = s.getLatestModuleInfo(ctx, modules[i].Path)
				return nil
			})
		}
		return nil
	})
	return modules, group.Wait()
}

func (s *Server) serveModules(w http.ResponseWriter, req *http.Request) {
	modules, err := s.getAllowedModulesInfo(req.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("error getting allowed modules info: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(modules)
}

func (s *Server) serveDashboard(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	modules, err := s.getAllowedModulesInfo(req.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("error getting allowed modules info: %s", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Xss-Protection", "0")
	w.WriteHeader(http.StatusOK)
	dashboardTemplate.Execute(w, modules)
}
