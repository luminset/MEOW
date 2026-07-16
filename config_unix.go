//go:build darwin || freebsd || linux || netbsd || openbsd
// +build darwin freebsd linux netbsd openbsd

package main

import (
	"path/filepath"
)

const (
	rcFname     = "rc"
	directFname = "direct"
	proxyFname  = "proxy"
	rejectFname = "reject"
	CNIPFname   = "china_ip_list"
	QQWryFname  = "QQWry.dat"

	newLine = "\n"
)

func getDefaultRcFile() string {
	return filepath.Join(getUserHomeDir(), ".meow", rcFname)
}
