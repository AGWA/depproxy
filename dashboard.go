package depproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"

	"golang.org/x/sync/errgroup"
	vulnclient "golang.org/x/vuln/client"
	"golang.org/x/vuln/osv"
	"src.agwa.name/depproxy/goproxy"
)

var dashboardTemplate = template.Must(template.ParseFS(content, "templates/dashboard.html"))

type allowedModuleInfo struct {
	AllowedModule
	CurrentInfo *goproxy.ModuleInfo
	CurrentErr  error
	Vulns       []*osv.Entry
	VulnsErr    error
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

func vulnAffects(entry *osv.Entry, version goproxy.ModuleVersion) bool {
	for _, affected := range entry.Affected {
		if affected.Ranges.AffectsSemver(version.String()) {
			return true
		}
	}
	return false
}

func (s *Server) getVulnsAffecting(ctx context.Context, module goproxy.ModulePath, version goproxy.ModuleVersion) ([]*osv.Entry, error) {
	client, err := vulnclient.NewClient([]string{"https://vuln.go.dev"}, vulnclient.Options{})
	if err != nil {
		return nil, err
	}
	allEntries, err := client.GetByModule(ctx, module.String())
	if err != nil {
		return nil, err
	}
	entries := allEntries[:0]
	for _, entry := range allEntries {
		if vulnAffects(entry, version) {
			entries = append(entries, entry)
		}
	}
	return entries, nil
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
				group.Go(func() error {
					modules[i].Vulns, modules[i].VulnsErr = s.getVulnsAffecting(ctx, modules[i].Path, modules[i].Version)
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
