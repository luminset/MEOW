package main

import (
	"os"
	"path/filepath"
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

	parser.ParseParentProbeURL("[2001:4860:4860::8888]:443")
	if config.ParentProbeURL != "[2001:4860:4860::8888]:443" {
		t.Fatalf("IPv6 parentProbeURL = %q", config.ParentProbeURL)
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
