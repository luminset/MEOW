package main

import (
	"net"
	"testing"
)

func TestIPShouldDirect(t *testing.T) {

	initCNIPData()

	blockedIPDomains := []string{
		"gist.github.com",
		"twitter.com",
	}
	for _, domain := range blockedIPDomains {
		hostIPs, err := net.LookupIP(domain)

		if err != nil {
			continue
		}

		var ip string
		ip = hostIPs[0].String()

		if ipShouldDirect(ip) {
			t.Errorf("ip %s should be considered using proxy, domain: %s", ip, domain)
		}
	}

	directIPDomains := []string{
		"baidu.com",
		"www.ahut.edu.cn",
		"bt.byr.cn",
	}
	for _, domain := range directIPDomains {
		hostIPs, err := net.LookupIP(domain)

		if err != nil {
			continue
		}

		var ip string
		ip = hostIPs[0].String()

		if !ipShouldDirect(ip) {
			t.Errorf("ip %s should be considered using direct, domain: %s", ip, domain)
		}
	}

}

func TestIPShouldDirectIPv6(t *testing.T) {

	initCNIPData()

	privateIPv6 := []string{
		"::1",
		"::",
		"fc00::1",
		"fd00::1",
		"fe80::1",
	}
	for _, ip := range privateIPv6 {
		if !ipShouldDirect(ip) {
			t.Errorf("private IPv6 %s should be considered using direct", ip)
		}
	}

	globalIPv6 := []string{
		"2001:4860:4860::8888",
		"2607:f8b0:4005:805::200e",
	}
	for _, ip := range globalIPv6 {
		if ipShouldDirect(ip) {
			t.Errorf("global IPv6 %s should be considered using proxy", ip)
		}
	}

}
