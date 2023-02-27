package depproxy

import (
	"fmt"
	"strings"

	"src.agwa.name/depproxy/goproxy"
)

func vcsDiff(vcs string, url string, oldOrigin, newOrigin *goproxy.ModuleOrigin) string {
	switch {
	case vcs == "git" && strings.HasPrefix(url, "https://github.com/"):
		return fmt.Sprintf("%s/compare/%s...%s", url, oldOrigin.Hash, newOrigin.Hash)
	default:
		return ""
	}
}
