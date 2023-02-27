package goproxy

import (
	"golang.org/x/mod/module"
)

type ModulePath string // holds a valid module path

func (path ModulePath) String() string {
	return string(path)
}

func (path ModulePath) Escaped() string {
	escaped, err := module.EscapePath(string(path))
	if err != nil {
		panic("goproxy.ModulePath contains an invalid path that could not be escaped: " + err.Error())
	}
	return escaped
}

func (path *ModulePath) UnmarshalText(text []byte) error {
	str := string(text)
	if err := module.CheckPath(str); err != nil {
		return err
	}
	*path = ModulePath(str)
	return nil
}

func UnescapeModulePath(escaped string) (ModulePath, error) {
	str, err := module.UnescapePath(escaped)
	return ModulePath(str), err
}

func MakeModulePath(str string) (ModulePath, error) {
	if err := module.CheckPath(str); err != nil {
		return "", err
	}
	return ModulePath(str), nil
}
