package goproxy

import (
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

type ModuleVersion string // holds a valid module version

func (version ModuleVersion) String() string {
	return string(version)
}

func (version ModuleVersion) Escaped() string {
	escaped, err := module.EscapeVersion(string(version))
	if err != nil {
		panic("goproxy.ModuleVersion contains an invalid version that could not be escaped: " + err.Error())
	}
	return escaped
}

func (version *ModuleVersion) UnmarshalText(text []byte) error {
	str := string(text)
	if _, err := module.EscapeVersion(str); err != nil {
		return err
	}
	*version = ModuleVersion(str)
	return nil
}

func UnescapeModuleVersion(escaped string) (ModuleVersion, error) {
	str, err := module.UnescapeVersion(escaped)
	return ModuleVersion(str), err
}

func MakeModuleVersion(str string) (ModuleVersion, error) {
	if _, err := module.EscapeVersion(str); err != nil {
		return "", err
	}
	return ModuleVersion(str), nil
}

func (version ModuleVersion) Compare(other ModuleVersion) int {
	return semver.Compare(string(version), string(other))
}
