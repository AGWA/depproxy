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
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"slices"
	"sync"

	"src.agwa.name/depproxy/internal/diff"
	"src.agwa.name/depproxy/internal/diff/myers"
	"src.agwa.name/depproxy/internal/goproxy"
)

var diffTemplate = template.Must(template.ParseFS(content, "templates/diff.html"))

func (s *Server) downloadUpstreamZip(ctx context.Context, module goproxy.ModulePath, version goproxy.ModuleVersion) (*zip.Reader, error) {
	resp, err := s.requestUpstream(ctx, module, goproxy.ZipRequest{Version: version})
	if err != nil {
		return nil, fmt.Errorf("error communicating with upstream proxy: %w", err)
	}
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("error communicating with upstream proxy: %w", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(respBody), int64(len(respBody)))
	if err != nil {
		return nil, fmt.Errorf("error reading module zip file: %w", err)
	}
	return reader, nil
}

type nullReadCloser struct{}

func (nullReadCloser) Read([]byte) (int, error) { return 0, io.EOF }
func (nullReadCloser) Close() error             { return nil }

func openNullReadCloser() (io.ReadCloser, error) { return nullReadCloser{}, nil }

func trimZipFilePrefix(filename string, module string, version string) string {
	return strings.TrimPrefix(filename, module+"@"+version+"/")
}

func makeFileDiff(oldLabel, newLabel string, openOldFile, openNewFile func() (io.ReadCloser, error)) (string, error) {
	oldFile, err := openOldFile()
	if err != nil {
		return "", fmt.Errorf("error opening %s: %w", oldLabel, err)
	}
	defer oldFile.Close()

	newFile, err := openNewFile()
	if err != nil {
		return "", fmt.Errorf("error opening %s: %w", newLabel, err)
	}
	defer newFile.Close()

	oldBytes, err := io.ReadAll(oldFile)
	if err != nil {
		return "", fmt.Errorf("error reading %s: %w", oldLabel, err)
	}
	newBytes, err := io.ReadAll(newFile)
	if err != nil {
		return "", fmt.Errorf("error reading %s: %w", newLabel, err)
	}

	//edits := diff.Strings(string(oldBytes), string(newBytes))
	edits := myers.ComputeEdits(string(oldBytes), string(newBytes))
	unified, err := diff.ToUnified(oldLabel, newLabel, string(oldBytes), edits)
	if err != nil {
		return "", fmt.Errorf("error making unified diff: %w", err)
	}
	return unified, nil
}

func makeDiff(module string, oldVer, newVer string, oldFiles, newFiles []*zip.File) (string, error) {
	slices.SortFunc(oldFiles, func(a, b *zip.File) int {
		return cmp.Compare(trimZipFilePrefix(a.Name, module, oldVer), trimZipFilePrefix(b.Name, module, oldVer))
	})
	slices.SortFunc(newFiles, func(a, b *zip.File) int {
		return cmp.Compare(trimZipFilePrefix(a.Name, module, newVer), trimZipFilePrefix(b.Name, module, newVer))
	})

	var fullDiff strings.Builder

	var oldPos, newPos int
	for oldPos < len(oldFiles) || newPos < len(newFiles) {
		var fileDiff string
		var err error
		if oldPos == len(oldFiles) || trimZipFilePrefix(newFiles[newPos].Name, module, newVer) < trimZipFilePrefix(oldFiles[oldPos].Name, module, oldVer) {
			// newFiles[newPos].Name not in oldFiles
			newFile := newFiles[newPos]
			fileDiff, err = makeFileDiff("/dev/null", newFile.Name, openNullReadCloser, newFile.Open)
			newPos++
		} else if newPos == len(newFiles) || trimZipFilePrefix(oldFiles[oldPos].Name, module, oldVer) < trimZipFilePrefix(newFiles[newPos].Name, module, newVer) {
			// oldFiles[oldPos].Name not in newFiles
			oldFile := oldFiles[oldPos]
			fileDiff, err = makeFileDiff(oldFile.Name, "/dev/null", oldFile.Open, openNullReadCloser)
			oldPos++
		} else {
			oldFile := oldFiles[oldPos]
			newFile := newFiles[newPos]
			fileDiff, err = makeFileDiff(oldFile.Name, newFile.Name, oldFile.Open, newFile.Open)

			newPos++
			oldPos++
		}
		if err != nil {
			return "", err
		}
		fullDiff.WriteString(fileDiff)
	}

	return fullDiff.String(), nil
}

func (s *Server) serveDiff(w http.ResponseWriter, req *http.Request) {
	module, err := goproxy.MakeModulePath(req.FormValue("module"))
	if err != nil {
		http.Error(w, "invalid module version: "+err.Error(), http.StatusBadRequest)
		return
	}
	oldVer, err := goproxy.MakeModuleVersion(req.FormValue("old"))
	if err != nil {
		http.Error(w, "invalid module version: "+err.Error(), http.StatusBadRequest)
		return
	}
	newVer, err := goproxy.MakeModuleVersion(req.FormValue("new"))
	if err != nil {
		http.Error(w, "invalid module version: "+err.Error(), http.StatusBadRequest)
		return
	}

	var oldZip, newZip *zip.Reader
	var oldErr, newErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		oldZip, oldErr = s.downloadUpstreamZip(req.Context(), module, oldVer)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		newZip, newErr = s.downloadUpstreamZip(req.Context(), module, newVer)
	}()
	wg.Wait()

	if errors.Is(oldErr, errNotFound) {
		http.Error(w, fmt.Sprintf("%s@%s not found at upstream proxy", module, oldVer), http.StatusNotFound)
		return
	} else if errors.Is(newErr, errNotFound) {
		http.Error(w, fmt.Sprintf("%s@%s not found at upstream proxy", module, newVer), http.StatusNotFound)
		return
	} else if oldErr != nil {
		http.Error(w, fmt.Sprintf("error downloading %s@%s from upstream proxy: %s", module, oldVer, oldErr), http.StatusBadGateway)
		return
	} else if newErr != nil {
		http.Error(w, fmt.Sprintf("error downloading %s@%s from upstream proxy: %s", module, newVer, newErr), http.StatusBadGateway)
		return
	}

	diff, err := makeDiff(module.String(), oldVer.String(), newVer.String(), oldZip.File, newZip.File)
	if err != nil {
		http.Error(w, fmt.Sprintf("error making diff: %s", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	//w.Header().Set("Content-Type", "text/x-diff; charset=UTF-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, diff)
}

func (s *Server) serveDiffHTML(w http.ResponseWriter, req *http.Request) {
	var (
		module = req.FormValue("module")
		oldVer = req.FormValue("old")
		newVer = req.FormValue("new")
	)
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Xss-Protection", "0")
	w.WriteHeader(http.StatusOK)
	diffTemplate.Execute(w, struct {
		Module string
		OldVer string
		NewVer string
	}{
		Module: module,
		OldVer: oldVer,
		NewVer: newVer,
	})
}
