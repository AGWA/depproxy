package goproxy

import (
	"time"
)

type ModuleInfo struct {
	Version ModuleVersion
	Time    time.Time
	Origin  *ModuleOrigin
}

type ModuleOrigin struct {
	VCS  string
	URL  string
	Ref  string
	Hash string
}
