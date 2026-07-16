package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseListen(t *testing.T) {
	parser := configParser{}
	parser.ParseListen("http://127.0.0.1:8888")

	hp, ok := listenProxy[0].(*httpProxy)
	if !ok {
		t.Error("listen http proxy type wrong")
	}
	if hp.addr != "127.0.0.1:8888" {
		t.Error("listen http server address parse error")
	}

	parser.ParseListen("http://127.0.0.1:8888 1.2.3.4:5678")
	hp, ok = listenProxy[1].(*httpProxy)
	if hp.addrInPAC != "1.2.3.4:5678" {
		t.Error("listen http addrInPAC parse error")
	}
}

func TestParseProxy(t *testing.T) {
	pool, ok := parentProxy.(*backupParentPool)
	if !ok {
		t.Fatal("parentPool by default should be backup pool")
	}
	cnt := -1

	var parser configParser
	parser.ParseProxy("http://127.0.0.1:8080")
	cnt++

	hp, ok := pool.parent[cnt].ParentProxy.(*httpParent)
	if !ok {
		t.Fatal("1st http proxy parsed not as httpParent")
	}
	if hp.server != "127.0.0.1:8080" {
		t.Error("1st http proxy server address wrong, got:", hp.server)
	}

	parser.ParseProxy("http://user:passwd@127.0.0.2:9090")
	cnt++
	hp, ok = pool.parent[cnt].ParentProxy.(*httpParent)
	if !ok {
		t.Fatal("2nd http proxy parsed not as httpParent")
	}
	if hp.server != "127.0.0.2:9090" {
		t.Error("2nd http proxy server address wrong, got:", hp.server)
	}
	if hp.authHeader == nil {
		t.Error("2nd http proxy server user password not parsed")
	}

	parser.ParseProxy("socks5://127.0.0.1:1080")
	cnt++
	sp, ok := pool.parent[cnt].ParentProxy.(*socksParent)
	if !ok {
		t.Fatal("socks proxy parsed not as socksParent")
	}
	if sp.server != "127.0.0.1:1080" {
		t.Error("socks server address wrong, got:", sp.server)
	}

	parser.ParseProxy("ss://aes-256-cfb:foobar!@127.0.0.1:1080")
	cnt++
	_, ok = pool.parent[cnt].ParentProxy.(*shadowsocksParent)
	if !ok {
		t.Fatal("shadowsocks proxy parsed not as shadowsocksParent")
	}
}

func TestParseProxyMode(t *testing.T) {
	parser := configParser{}

	parser.ParseProxyMode("default")
	if config.ProxyMode != proxyModeDefault {
		t.Error("proxyMode default parse error")
	}

	parser.ParseProxyMode("keep")
	if config.ProxyMode != proxyModeKeep {
		t.Error("proxyMode keep parse error")
	}

	parser.ParseProxyMode("cow")
	if config.ProxyMode != proxyModeCow {
		t.Error("proxyMode cow parse error")
	}
}

func TestParseParentProbeOptions(t *testing.T) {
	oldParentProbeURL := parentProbeURL
	oldConfigParentProbeURL := config.ParentProbeURL
	oldParentProbeInterval := config.ParentProbeInterval
	defer func() {
		parentProbeURL = oldParentProbeURL
		config.ParentProbeURL = oldConfigParentProbeURL
		config.ParentProbeInterval = oldParentProbeInterval
	}()

	parser := configParser{}
	parser.ParseParentProbeURL("example.com:443")
	if config.ParentProbeURL != "example.com:443" {
		t.Fatalf("parentProbeURL = %q, want example.com:443", config.ParentProbeURL)
	}
	if parentProbeURL.Host != "example.com" || parentProbeURL.Port != "443" || parentProbeURL.Domain != "example.com" {
		t.Fatalf("parentProbeURL parsed wrong: %+v", parentProbeURL)
	}

	parser.ParseParentProbeURL("1.1.1.1:443")
	if config.ParentProbeURL != "1.1.1.1:443" {
		t.Fatalf("IPv4 parentProbeURL = %q", config.ParentProbeURL)
	}

	parser.ParseParentProbeURL("[2001:4860:4860::8888]:443")
	if config.ParentProbeURL != "[2001:4860:4860::8888]:443" {
		t.Fatalf("IPv6 parentProbeURL = %q", config.ParentProbeURL)
	}

	parser.ParseParentProbeURL("")
	if config.ParentProbeURL != defaultParentProbeURL {
		t.Fatalf("empty parentProbeURL = %q, want %q", config.ParentProbeURL, defaultParentProbeURL)
	}

	parser.ParseParentProbeInterval("30s")
	if config.ParentProbeInterval != 30*time.Second {
		t.Fatalf("parentProbeInterval = %s, want 30s", config.ParentProbeInterval)
	}

	parser.ParseParentProbeInterval("1s")
	if config.ParentProbeInterval != defaultParentProbeInterval {
		t.Fatalf("small parentProbeInterval = %s, want %s", config.ParentProbeInterval, defaultParentProbeInterval)
	}
}

func TestParseQQWryOptionAliases(t *testing.T) {
	oldQQWryFile := config.QQWryFile
	oldQQWryUpdateURL := config.QQWryUpdateURL
	oldQQWryUpdateInterval := config.QQWryUpdateInterval
	defer func() {
		config.QQWryFile = oldQQWryFile
		config.QQWryUpdateURL = oldQQWryUpdateURL
		config.QQWryUpdateInterval = oldQQWryUpdateInterval
	}()

	parser := configParser{}
	parser.ParseQqwryFile("QQWry.dat")
	if filepath.Base(config.QQWryFile) != "QQWry.dat" {
		t.Fatalf("qqwryFile alias parse failed: %q", config.QQWryFile)
	}
	parser.ParseQqwryUpdateURL("https://example.com/qqwry.dat")
	if config.QQWryUpdateURL != "https://example.com/qqwry.dat" {
		t.Fatalf("qqwryUpdateURL alias parse failed: %q", config.QQWryUpdateURL)
	}
	parser.ParseQqwryUpdateInterval("72h")
	if config.QQWryUpdateInterval != 72*time.Hour {
		t.Fatalf("qqwryUpdateInterval alias parse failed: %s", config.QQWryUpdateInterval)
	}
}

func TestParseFileOptionsRelativeToConfigDir(t *testing.T) {
	oldDir := config.dir
	oldLogFile := config.LogFile
	oldDirectFile := config.DirectFile
	oldProxyFile := config.ProxyFile
	oldRejectFile := config.RejectFile
	oldUserPasswdFile := config.UserPasswdFile
	oldQQWryFile := config.QQWryFile
	oldCert := config.Cert
	oldKey := config.Key
	defer func() {
		config.dir = oldDir
		config.LogFile = oldLogFile
		config.DirectFile = oldDirectFile
		config.ProxyFile = oldProxyFile
		config.RejectFile = oldRejectFile
		config.UserPasswdFile = oldUserPasswdFile
		config.QQWryFile = oldQQWryFile
		config.Cert = oldCert
		config.Key = oldKey
	}()

	config.dir = t.TempDir()
	parser := configParser{}

	parser.ParseLogFile("meow.log")
	if want := filepath.Join(config.dir, "meow.log"); config.LogFile != want {
		t.Fatalf("logFile relative path = %q, want %q", config.LogFile, want)
	}

	parser.ParseDirectFile("direct.txt")
	if want := filepath.Join(config.dir, "direct.txt"); config.DirectFile != want {
		t.Fatalf("directFile relative path = %q, want %q", config.DirectFile, want)
	}

	parser.ParseProxyFile("proxy.txt")
	if want := filepath.Join(config.dir, "proxy.txt"); config.ProxyFile != want {
		t.Fatalf("proxyFile relative path = %q, want %q", config.ProxyFile, want)
	}

	parser.ParseRejectFile("reject.txt")
	if want := filepath.Join(config.dir, "reject.txt"); config.RejectFile != want {
		t.Fatalf("rejectFile relative path = %q, want %q", config.RejectFile, want)
	}

	userPasswdFile := filepath.Join(config.dir, "user_passwd.txt")
	if err := os.WriteFile(userPasswdFile, []byte("user:passwd\n"), 0644); err != nil {
		t.Fatal(err)
	}
	parser.ParseUserPasswdFile("user_passwd.txt")
	if config.UserPasswdFile != userPasswdFile {
		t.Fatalf("userPasswdFile relative path = %q, want %q", config.UserPasswdFile, userPasswdFile)
	}

	parser.ParseQQWryFile("QQWry.dat")
	want := filepath.Join(config.dir, "QQWry.dat")
	if config.QQWryFile != want {
		t.Fatalf("qqwryFile relative path = %q, want %q", config.QQWryFile, want)
	}

	parser.ParseCert("cert.pem")
	if want := filepath.Join(config.dir, "cert.pem"); config.Cert != want {
		t.Fatalf("cert relative path = %q, want %q", config.Cert, want)
	}

	parser.ParseKey("key.pem")
	if want := filepath.Join(config.dir, "key.pem"); config.Key != want {
		t.Fatalf("key relative path = %q, want %q", config.Key, want)
	}

	abs := filepath.Join(filepath.VolumeName(config.dir), string(filepath.Separator), "data", "QQWry.dat")
	if !filepath.IsAbs(abs) {
		abs, _ = filepath.Abs(abs)
	}
	parser.ParseQQWryFile(abs)
	if config.QQWryFile != abs {
		t.Fatalf("qqwryFile absolute path = %q, want %q", config.QQWryFile, abs)
	}
}

func TestSyncConfigOptionsDeduplicatesGeneratedBlocks(t *testing.T) {
	rc := filepath.Join(t.TempDir(), "rc")
	content := strings.Join([]string{
		"# 自定义配置",
		"listen = http://127.0.0.1:5555",
		"proxyMode = keep",
		"proxy = http://127.0.0.1:8080",
		"proxy = socks5://127.0.0.1:1080",
		"directFallbackStatus = 403,451",
		"parentFallbackStatus = 502,503,504",
		"#parentProbeURL = 1.1.1.1:443",
		"",
		"#############################",
		"# 以下配置项由当前版本自动补全，请按需取消注释并修改",
		"#############################",
		"#############################",
		"# 二级代理连通性/延迟探测地址，仅 loadBalance = latency 时使用",
		"#############################",
		"#parentProbeURL = www.google.com:443",
		"",
		"#############################",
		"# 以下配置项由当前版本自动补全，请按需取消注释并修改",
		"#############################",
		"#############################",
		"# 二级代理连通性/延迟探测地址，仅 loadBalance = latency 时使用",
		"#############################",
		"#parentProbeURL = www.google.com:443",
		"",
	}, "\n")
	if err := os.WriteFile(rc, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	syncConfigOptions(rc)
	first, err := os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	firstText := string(first)
	if strings.Contains(firstText, "以下配置项由当前版本自动补全") {
		t.Fatal("auto completion header should be cleaned")
	}
	for _, want := range []string{
		"listen = http://127.0.0.1:5555",
		"proxyMode = keep",
		"proxy = http://127.0.0.1:8080",
		"proxy = socks5://127.0.0.1:1080",
		"directFallbackStatus = 403,451",
		"parentFallbackStatus = 502,503,504",
	} {
		if !strings.Contains(firstText, want) {
			t.Fatalf("rebuilt config should contain %q, got:\n%s", want, firstText)
		}
	}
	if countConfigOptionKey(firstText, "parentProbeURL") != 1 {
		t.Fatalf("parentProbeURL template should appear once, got config:\n%s", firstText)
	}
	if countConfigOptionKey(firstText, "proxy") != 2 {
		t.Fatalf("proxy should preserve two active values, got config:\n%s", firstText)
	}

	syncConfigOptions(rc)
	second, err := os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != firstText {
		t.Fatalf("syncConfigOptions should be idempotent\nfirst:\n%s\nsecond:\n%s", firstText, string(second))
	}
}

func TestParseFallbackStatusOptions(t *testing.T) {
	oldDirect := config.DirectFallbackStatus
	oldParent := config.ParentFallbackStatus
	oldHTTPCode := config.HttpErrorCode
	oldMode := config.ProxyMode
	oldParentProxy := parentProxy
	defer func() {
		config.DirectFallbackStatus = oldDirect
		config.ParentFallbackStatus = oldParent
		config.HttpErrorCode = oldHTTPCode
		config.ProxyMode = oldMode
		parentProxy = oldParentProxy
	}()

	parser := configParser{}
	parser.ParseDirectFallbackStatus("403, 451")
	if !shouldDirectFallbackStatus(403) || !shouldDirectFallbackStatus(451) || shouldDirectFallbackStatus(503) {
		t.Fatalf("directFallbackStatus parse failed: %#v", config.DirectFallbackStatus)
	}

	parser.ParseHttpErrorCode("418")
	if !shouldDirectFallbackStatus(418) {
		t.Fatal("httpErrorCode should remain compatible as direct fallback status")
	}
	parentProxy = &backupParentPool{parent: []ParentWithFail{{ParentProxy: &httpParent{server: "127.0.0.1:1"}}}}

	parser.ParseParentFallbackStatus("502,503,504")
	config.ProxyMode = proxyModeKeep
	if !shouldParentFallbackStatus(503) {
		t.Fatalf("parentFallbackStatus parse failed: %#v", config.ParentFallbackStatus)
	}
	if !canFallbackHTTPStatus(&Request{}, false, 503, CustomHttpErr) {
		t.Fatal("KEEP mode parent 503 should be able to fallback to direct")
	}
	if canFallbackHTTPStatus(&Request{fallback: true}, false, 503, CustomHttpErr) {
		t.Fatal("request should not fallback more than once")
	}

	directSV := &serverConn{Conn: directConn{}}
	if !shouldInterceptHTTPStatus(directSV, 403) {
		t.Fatal("direct 403 should be intercepted before sending response")
	}
	parentSV := &serverConn{Conn: httpConn{}}
	if !shouldInterceptHTTPStatus(parentSV, 503) {
		t.Fatal("KEEP parent 503 should be intercepted before sending response")
	}
	config.ProxyMode = proxyModeDefault
	if shouldInterceptHTTPStatus(parentSV, 503) {
		t.Fatal("parent 503 should not be intercepted outside KEEP mode")
	}
}

func TestFallbackStatusConflictDetection(t *testing.T) {
	oldDirect := config.DirectFallbackStatus
	oldParent := config.ParentFallbackStatus
	oldHTTPCode := config.HttpErrorCode
	defer func() {
		config.DirectFallbackStatus = oldDirect
		config.ParentFallbackStatus = oldParent
		config.HttpErrorCode = oldHTTPCode
	}()

	config.DirectFallbackStatus = map[int]bool{403: true}
	config.HttpErrorCode = 503
	config.ParentFallbackStatus = defaultParentFallbackStatus()
	conflict := intersectStatusCodes(directFallbackStatusSet(), config.ParentFallbackStatus)
	if !conflict[503] || len(conflict) != 1 {
		t.Fatalf("expected only 503 conflict, got %#v", conflict)
	}

	config.HttpErrorCode = 0
	config.DirectFallbackStatus = map[int]bool{403: true}
	conflict = intersectStatusCodes(directFallbackStatusSet(), config.ParentFallbackStatus)
	if len(conflict) != 0 {
		t.Fatalf("expected no conflict, got %#v", conflict)
	}
}

func countConfigOptionKey(data, want string) int {
	keys := configOptionKeySet()
	var n int
	for _, line := range splitConfigLines(data) {
		if key, _, ok := configOptionKeyFromLine(line, keys); ok && key == want {
			n++
		}
	}
	return n
}
