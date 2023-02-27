package depproxy

import (
	"context"
	"embed"
	"errors"
	"net/http"
	"net/url"

	"src.agwa.name/depproxy/goproxy"
)

//go:embed assets/* templates/*
var content embed.FS

var errNotFound = errors.New("not found")

type Server struct {
	UpstreamProxy  *url.URL
	AllowedModules []AllowedModule
}

func (s *Server) getAllowedVersionPattern(path string) string {
	for _, m := range s.AllowedModules {
		if matched, _ := m.matchesPath(path); matched {
			return m.VersionPattern
		}
	}
	return ""
}

func (s *Server) isModuleAllowed(path, version string) bool {
	for _, m := range s.AllowedModules {
		if matched, _ := m.matches(path, version); matched {
			return true
		}
	}
	return false
}

func (s *Server) redirectUpstream(w http.ResponseWriter, module goproxy.ModulePath, req goproxy.Request) {
	url := s.UpstreamProxy.JoinPath(module.Escaped(), req.Path())
	w.Header().Set("Location", url.String())
	w.WriteHeader(http.StatusSeeOther)
}

func (s *Server) requestUpstream(ctx context.Context, module goproxy.ModulePath, req goproxy.Request) (*http.Response, error) {
	url := s.UpstreamProxy.JoinPath(module.Escaped(), req.Path())
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusOK {
		return resp, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return nil, errNotFound
	} else {
		return nil, errors.New(url.String() + ": " + resp.Status)
	}
}

func (s *Server) HTTPHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/assets/", http.FileServer(http.FS(content)))
	mux.HandleFunc("/diff", s.serveDiff)
	mux.HandleFunc("/diff.html", s.serveDiffHTML)
	mux.HandleFunc("/modules", s.serveModules)
	mux.HandleFunc("/proxy/", s.serveProxyRequest)
	mux.HandleFunc("/", s.serveDashboard)
	return mux
}
