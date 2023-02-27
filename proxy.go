package depproxy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"src.agwa.name/depproxy/goproxy"
)

func (s *Server) requestListFromUpstream(ctx context.Context, module goproxy.ModulePath) ([]string, error) {
	resp, err := s.requestUpstream(ctx, module, listRequest{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	versions := []string{}
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		versions = append(versions, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return versions, nil
}

func (s *Server) serveLatestRequest(ctx context.Context, w http.ResponseWriter, module goproxy.ModulePath) {
	allowedVersionPattern := s.getAllowedVersionPattern(module.String())
	if allowedVersionPattern == "" {
		http.Error(w, fmt.Sprintf("Module %q is not allowed", module), http.StatusForbidden)
	} else if allowedVersionPattern == "*" {
		s.redirectUpstream(w, module, latestRequest{})
	} else if version, err := goproxy.MakeModuleVersion(allowedVersionPattern); err == nil {
		s.redirectUpstream(w, module, infoRequest{Version: version})
	} else {
		http.Error(w, fmt.Sprintf("Module %q has allowed version: %s", module, err), http.StatusInternalServerError)
	}
}

func (s *Server) serveListRequest(ctx context.Context, w http.ResponseWriter, module goproxy.ModulePath) {
	versions, err := s.requestListFromUpstream(ctx, module)
	if errors.Is(err, errNotFound) {
		http.Error(w, "Module not found at upstream proxy", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Error communicating with upstream proxy: "+err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	for _, version := range versions {
		if s.isModuleAllowed(module.String(), version) {
			fmt.Fprintln(w, version)
		}
	}
}

func (s *Server) serveProxyRequest(w http.ResponseWriter, httpReq *http.Request) {
	if strings.HasPrefix(httpReq.URL.Path, "/proxy/sumdb/") {
		http.Error(w, "sumdb is not proxied", http.StatusNotFound)
		return
	}
	module, request, err := parseProxyPath(strings.TrimPrefix(httpReq.URL.Path, "/proxy/"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch request := request.(type) {
	case latestRequest:
		s.serveLatestRequest(httpReq.Context(), w, module)
	case listRequest:
		s.serveListRequest(httpReq.Context(), w, module)
	case infoRequest:
		s.redirectUpstream(w, module, request)
	case modRequest:
		s.redirectUpstream(w, module, request)
	case zipRequest:
		if s.isModuleAllowed(module.String(), request.Version.String()) {
			s.redirectUpstream(w, module, request)
		} else {
			http.Error(w, fmt.Sprintf("Version %q of module %q is not allowed", request.Version, module), http.StatusForbidden)
		}
	default:
		http.Error(w, "Unsupported request", http.StatusBadRequest)
	}
}
