package main

import (
	"bytes"
	"net"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/cyfdecyf/bufio"
)

type DomainList struct {
	Domain         map[string]DomainType
	IP             map[string]DomainType
	IPNetRules     []ipNetRule
	IPRangeRules   []ipRangeRule
	WildcardDomain []matchRule
	PathRules      []matchRule
	sync.RWMutex
}

type DomainType byte

const (
	domainTypeUnknown DomainType = iota
	domainTypeDirect
	domainTypeProxy
	domainTypeReject
)

type matchRule struct {
	Pattern    string
	DomainType DomainType
	Wildcard   bool
}

type ipNetRule struct {
	Network    *net.IPNet
	DomainType DomainType
}

type ipRangeRule struct {
	Start      net.IP
	End        net.IP
	DomainType DomainType
}

func newDomainList() *DomainList {
	return &DomainList{
		Domain: map[string]DomainType{},
		IP:     map[string]DomainType{},
	}
}

func (domainList *DomainList) judge(url *URL) (domainType DomainType) {
	debug.Printf("judging host: %s", url.Host)
	if domainList.match(url, domainTypeReject) {
		debug.Printf("url should reject")
		return domainTypeReject
	}
	if parentProxy.empty() { // no way to retry, so always visit directly
		return domainTypeDirect
	}
	if domainList.match(url, domainTypeDirect) {
		debug.Printf("url should direct")
		return domainTypeDirect
	}
	if domainList.match(url, domainTypeProxy) {
		debug.Printf("url should using proxy")
		return domainTypeProxy
	}
	if config.ProxyMode == proxyModeCow {
		debug.Printf("cow mode defaults to direct")
		return domainTypeDirect
	}
	if url.Domain == "" { // simple host or private ip
		return domainTypeDirect
	}

	if !config.JudgeByIP {
		return domainTypeProxy
	}
	debug.Printf("judging by ip")
	var ip string
	isIP, isPrivate := hostIsIP(url.Host)
	if isIP {
		if isPrivate {
			domainList.add(url.Host, domainTypeDirect)
			return domainTypeDirect
		}
		ip = url.Host
	} else {
		hostIPs, err := net.LookupIP(url.Host)
		if err != nil {
			errl.Printf("error looking up host ip %s, err %s", url.Host, err)
			return domainTypeProxy
		}
		ip = hostIPs[0].String()
	}

	if ipShouldDirect(ip) {
		domainList.add(url.Host, domainTypeDirect)
		debug.Printf("host or domain should direct")
		return domainTypeDirect
	} else {
		domainList.add(url.Host, domainTypeProxy)
		debug.Printf("host or domain should using proxy")
		return domainTypeProxy
	}
}

func (domainList *DomainList) add(host string, domainType DomainType) {
	domainList.Lock()
	defer domainList.Unlock()
	if oldType := domainList.Domain[host]; oldType == domainTypeReject && domainType != domainTypeReject {
		return
	}
	domainList.Domain[host] = domainType
}

func (domainList *DomainList) GetDomainList() []string {
	lst := make([]string, 0)
	for site, domainType := range domainList.Domain {
		if domainType == domainTypeDirect {
			lst = append(lst, site)
		}
	}
	return lst
}

var domainList = newDomainList()

func normalizeListHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	host = trimLastDot(host)
	return strings.ToLower(host)
}

func normalizeListIP(ip net.IP) net.IP {
	if ip == nil {
		return nil
	}
	if ip4 := ip.To4(); ip4 != nil {
		return ip4
	}
	return ip.To16()
}

func compareIP(a, b net.IP) int {
	a = normalizeListIP(a)
	b = normalizeListIP(b)
	if a == nil || b == nil || len(a) != len(b) {
		return -1
	}
	return bytes.Compare(a, b)
}

func hasWildcard(s string) bool {
	return strings.ContainsAny(s, "*?")
}

func wildcardMatch(pattern, value string) bool {
	ok, err := path.Match(pattern, value)
	if err == nil {
		return ok
	}
	return strings.Contains(value, strings.ReplaceAll(strings.ReplaceAll(pattern, "*", ""), "?", ""))
}

func pathRuleMatch(rule matchRule, pathValue string) bool {
	if pathValue == "" {
		return false
	}
	pathValue = strings.ToLower(pathValue)
	if rule.Wildcard {
		return wildcardMatch(rule.Pattern, pathValue)
	}
	return strings.Contains(pathValue, rule.Pattern)
}

func (domainList *DomainList) matchIP(ip net.IP, domainType DomainType) bool {
	ip = normalizeListIP(ip)
	if ip == nil {
		return false
	}
	if domainList.IP[ip.String()] == domainType {
		return true
	}
	for _, rule := range domainList.IPNetRules {
		if rule.DomainType == domainType && rule.Network.Contains(ip) {
			return true
		}
	}
	for _, rule := range domainList.IPRangeRules {
		if rule.DomainType == domainType && compareIP(rule.Start, ip) <= 0 && compareIP(ip, rule.End) <= 0 {
			return true
		}
	}
	return false
}

func (domainList *DomainList) match(url *URL, domainType DomainType) bool {
	if url == nil {
		return false
	}
	host := normalizeListHost(url.Host)
	domain := normalizeListHost(url.Domain)
	domainList.RLock()
	defer domainList.RUnlock()

	if ip := net.ParseIP(host); ip != nil && domainList.matchIP(ip, domainType) {
		return true
	}
	if host != "" && domainList.Domain[host] == domainType {
		return true
	}
	if domain != "" && domainList.Domain[domain] == domainType {
		return true
	}
	for _, rule := range domainList.WildcardDomain {
		if rule.DomainType == domainType && wildcardMatch(rule.Pattern, host) {
			return true
		}
	}
	for _, rule := range domainList.PathRules {
		if rule.DomainType == domainType && pathRuleMatch(rule, url.Path) {
			return true
		}
	}
	return false
}

func (domainList *DomainList) addRuleLocked(rule string, domainType DomainType) {
	rule = strings.TrimSpace(rule)
	if rule == "" || strings.HasPrefix(rule, "#") {
		return
	}
	rule = strings.ToLower(rule)

	if strings.HasPrefix(rule, "/") || strings.HasPrefix(rule, "?") {
		domainList.PathRules = append(domainList.PathRules, matchRule{
			Pattern:    rule,
			DomainType: domainType,
			Wildcard:   hasWildcard(rule),
		})
		return
	}

	if strings.Contains(rule, "://") {
		if url, err := ParseRequestURI(rule); err == nil {
			if url.Host != "" {
				domainList.addRuleLocked(url.Host, domainType)
			}
			if url.Path != "" {
				domainList.addRuleLocked(url.Path, domainType)
			}
			return
		}
	}

	if ip := net.ParseIP(rule); ip != nil {
		ip = normalizeListIP(ip)
		domainList.IP[ip.String()] = domainType
		return
	}

	if _, ipNet, err := net.ParseCIDR(rule); err == nil {
		domainList.IPNetRules = append(domainList.IPNetRules, ipNetRule{
			Network:    ipNet,
			DomainType: domainType,
		})
		return
	}

	if parts := strings.Split(rule, "-"); len(parts) == 2 {
		start := normalizeListIP(net.ParseIP(strings.TrimSpace(parts[0])))
		end := normalizeListIP(net.ParseIP(strings.TrimSpace(parts[1])))
		if start != nil && end != nil && len(start) == len(end) {
			if compareIP(start, end) > 0 {
				start, end = end, start
			}
			domainList.IPRangeRules = append(domainList.IPRangeRules, ipRangeRule{
				Start:      start,
				End:        end,
				DomainType: domainType,
			})
			return
		}
	}

	rule = normalizeListHost(rule)
	if hasWildcard(rule) {
		domainList.WildcardDomain = append(domainList.WildcardDomain, matchRule{
			Pattern:    rule,
			DomainType: domainType,
			Wildcard:   true,
		})
		return
	}

	debug.Printf("Loaded domain %s as type %v", rule, domainType)
	domainList.Domain[rule] = domainType
}

func initDomainList(domainListFile string, domainType DomainType) {
	var err error
	if err = isFileExists(domainListFile); err != nil {
		return
	}
	f, err := os.Open(domainListFile)
	if err != nil {
		errl.Println("Error opening domain list:", err)
		return
	}
	defer f.Close()

	domainList.Lock()
	defer domainList.Unlock()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		domain := strings.TrimSpace(scanner.Text())
		domainList.addRuleLocked(domain, domainType)
	}
	if scanner.Err() != nil {
		errl.Printf("Error reading domain list %s: %v\n", domainListFile, scanner.Err())
	}
}
