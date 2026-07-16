package main

import (
	"testing"
)

func TestJudge(t *testing.T) {
	domainList := newDomainList()

	domainList.Domain["com.cn"] = domainTypeDirect
	domainList.Domain["edu.cn"] = domainTypeDirect
	domainList.Domain["baidu.com"] = domainTypeDirect

	g, _ := ParseRequestURI("gtemp.com")
	if domainList.judge(g) == domainTypeProxy {
		t.Error("never visited site should be considered using proxy")
	}

	directDomains := []string{
		"baidu.com",
		"www.baidu.com",
		"www.ahut.edu.cn",
	}
	for _, domain := range directDomains {
		url, _ := ParseRequestURI(domain)
		if domainList.judge(url) == domainTypeDirect {
			t.Errorf("domain %s in direct list should be considered using direct, host: %s", domain, url.Host)
		}
	}

}

func TestDomainListEnhancedRules(t *testing.T) {
	domainList := newDomainList()
	rules := []string{
		"203.0.113.8",
		"198.51.100.0/24",
		"192.0.2.10-192.0.2.20",
		"www.example.com",
		"example.org",
		"*.ads.example.net",
		"/ad/",
		"?ad.js",
	}
	domainList.Lock()
	for _, rule := range rules {
		domainList.addRuleLocked(rule, domainTypeReject)
	}
	domainList.Unlock()

	blockedURLs := []string{
		"http://203.0.113.8/",
		"http://198.51.100.23/",
		"http://192.0.2.15/",
		"http://www.example.com/",
		"http://static.example.org/",
		"http://img.ads.example.net/",
		"http://assets.safe.test/static/ad/banner.js",
		"http://assets.safe.test?ad.js",
	}
	for _, rawURL := range blockedURLs {
		url, err := ParseRequestURI(rawURL)
		if err != nil {
			t.Fatalf("%s parse error: %v", rawURL, err)
		}
		if !domainList.match(url, domainTypeReject) {
			t.Errorf("%s should match reject rule", rawURL)
		}
	}

	allowedURLs := []string{
		"http://203.0.113.9/",
		"http://198.51.101.23/",
		"http://192.0.2.21/",
		"http://example.com/",
		"http://ads.example.net/",
		"http://assets.safe.test/static/ad-banner.js",
	}
	for _, rawURL := range allowedURLs {
		url, err := ParseRequestURI(rawURL)
		if err != nil {
			t.Fatalf("%s parse error: %v", rawURL, err)
		}
		if domainList.match(url, domainTypeReject) {
			t.Errorf("%s should not match reject rule", rawURL)
		}
	}
}
