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
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"src.agwa.name/depproxy/internal/goproxy"
)

func (s *Server) requestListFromUpstream(ctx context.Context, module goproxy.ModulePath) ([]goproxy.ModuleVersion, error) {
	resp, err := s.requestUpstream(ctx, module, goproxy.ListRequest{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	versions := []goproxy.ModuleVersion{}
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		version, err := goproxy.MakeModuleVersion(scanner.Text())
		if err != nil {
			return nil, fmt.Errorf("malformed version in list response: %w", err)
		}
		versions = append(versions, version)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return versions, nil
}

func (s *Server) serveLatestRequest(ctx context.Context, w http.ResponseWriter, module goproxy.ModulePath) {
	allowedModule := s.getAllowedModule(module)
	if allowedModule == nil {
		http.Error(w, fmt.Sprintf("Module %q is not allowed", module), http.StatusForbidden)
	} else if allowedModule.Version.IsEmpty() {
		s.redirectUpstream(w, module, goproxy.LatestRequest{})
	} else {
		s.redirectUpstream(w, module, goproxy.InfoRequest{Version: allowedModule.Version})
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
		if s.isModuleAllowed(module, version) {
			fmt.Fprintln(w, version)
		}
	}
}

func (s *Server) serveProxyRequest(w http.ResponseWriter, httpReq *http.Request) {
	if strings.HasPrefix(httpReq.URL.Path, "/proxy/sumdb/") {
		http.Error(w, "sumdb is not proxied", http.StatusNotFound)
		return
	}
	module, request, err := goproxy.ParseRequestPath(strings.TrimPrefix(httpReq.URL.Path, "/proxy/"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch request := request.(type) {
	case goproxy.LatestRequest:
		s.serveLatestRequest(httpReq.Context(), w, module)
	case goproxy.ListRequest:
		s.serveListRequest(httpReq.Context(), w, module)
	case goproxy.InfoRequest:
		s.redirectUpstream(w, module, request)
	case goproxy.ModRequest:
		s.redirectUpstream(w, module, request)
	case goproxy.ZipRequest:
		if s.isModuleAllowed(module, request.Version) {
			s.redirectUpstream(w, module, request)
		} else {
			http.Error(w, fmt.Sprintf("Version %q of module %q is not allowed", request.Version, module), http.StatusForbidden)
		}
	default:
		http.Error(w, "Unsupported request", http.StatusBadRequest)
	}
}
