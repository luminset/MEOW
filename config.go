package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/cyfdecyf/bufio"
)

const (
	sourceVersion     = "1.5"
	defaultListenAddr = "127.0.0.1:4411"
)

var (
	nohaFixBuild = "0"
	version      = sourceVersion + "-nohafix" + nohaFixBuild
)

type LoadBalanceMode byte

const (
	loadBalanceBackup LoadBalanceMode = iota
	loadBalanceHash
	loadBalanceLatency
)

type ProxyMode byte

const (
	proxyModeDefault ProxyMode = iota
	proxyModeKeep
	proxyModeCow
)

type Config struct {
	RcFile      string // config file
	LogFile     string // path for log file
	JudgeByIP   bool
	LoadBalance LoadBalanceMode // select load balance mode
	ProxyMode   ProxyMode       // select proxy decision/fallback mode

	SshServer []string

	// authenticate client
	UserPasswd     string
	UserPasswdFile string // file that contains user:passwd:[port] pairs
	AllowedClient  string
	AuthTimeout    time.Duration

	// advanced options
	DialTimeout time.Duration
	ReadTimeout time.Duration

	ParentFailureFeedback bool
	ParentProbeFailStatus map[int]bool
	ParentProbeURL        string
	ParentProbeInterval   time.Duration

	Core int

	HttpErrorCode int

	dir        string // directory containing config file
	DirectFile string // direct sites specified by user
	ProxyFile  string // sites using proxy specified by user
	RejectFile string
	CNIPFile   string

	QQWryFile           string
	QQWryUpdateURL      string
	QQWryUpdateInterval time.Duration

	// not configurable in config file
	PrintVer bool

	// not config option
	saveReqLine bool // for http and meow parent, should save request line from client
	Cert        string
	Key         string
}

var config Config
var configNeedUpgrade bool // whether should upgrade config file

func printVersion() {
	fmt.Println("MEOW version", version)
}

func initConfig(rcFile string) {
	config.dir = filepath.Dir(rcFile)
	config.DirectFile = filepath.Join(config.dir, directFname)
	config.ProxyFile = filepath.Join(config.dir, proxyFname)
	config.RejectFile = filepath.Join(config.dir, rejectFname)
	config.CNIPFile = filepath.Join(config.dir, CNIPFname)
	config.QQWryFile = filepath.Join(config.dir, QQWryFname)
	config.QQWryUpdateURL = defaultQQWryUpdateURL
	config.QQWryUpdateInterval = 24 * time.Hour

	config.JudgeByIP = true
	config.ProxyMode = proxyModeDefault
	config.ParentProbeURL = defaultParentProbeURL
	config.ParentProbeInterval = defaultParentProbeInterval

	config.AuthTimeout = 2 * time.Hour
}

// Whether command line options specifies listen addr
var cmdHasListenAddr bool

func parseCmdLineConfig() *Config {
	var c Config
	var listenAddr string

	flag.StringVar(&c.RcFile, "rc", "", "config file, defaults to $HOME/.meow/rc on Unix, ./rc.txt on Windows")
	// Specifying listen default value to StringVar would override config file options
	flag.StringVar(&listenAddr, "listen", "", "listen address, disables listen in config")
	flag.IntVar(&c.Core, "core", 2, "number of cores to use")
	flag.StringVar(&c.LogFile, "logFile", "", "write output to file")
	flag.BoolVar(&c.PrintVer, "version", false, "print version")
	flag.StringVar(&c.Cert, "cert", "", "cert for local https proxy")
	flag.StringVar(&c.Key, "key", "", "key for local https proxy")

	flag.Parse()

	if c.RcFile == "" {
		c.RcFile = getDefaultRcFile()
	} else {
		c.RcFile = expandTilde(c.RcFile)
	}
	initConfig(c.RcFile)
	ensureConfigFiles(c.RcFile)
	initDomainList(config.DirectFile, domainTypeDirect)
	initDomainList(config.ProxyFile, domainTypeProxy)
	initDomainList(config.RejectFile, domainTypeReject)

	if listenAddr != "" {
		configParser{}.ParseListen(listenAddr)
		cmdHasListenAddr = true // must come after parse
	}
	return &c
}

func parseBool(v, msg string) bool {
	switch v {
	case "true":
		return true
	case "false":
		return false
	default:
		Fatalf("%s should be true or false\n", msg)
	}
	return false
}

func parseInt(val, msg string) (i int) {
	var err error
	if i, err = strconv.Atoi(val); err != nil {
		Fatalf("%s should be an integer\n", msg)
	}
	return
}

func parseDuration(val, msg string) (d time.Duration) {
	var err error
	if d, err = time.ParseDuration(val); err != nil {
		Fatalf("%s %v\n", msg, err)
	}
	return
}

func allowEmptyConfigValue(key string) bool {
	switch key {
	case "shadowMethod", "logFile", "parentProbeURL":
		return true
	default:
		return false
	}
}

func checkServerAddr(addr string) error {
	_, _, err := net.SplitHostPort(addr)
	return err
}

func isUserPasswdValid(val string) bool {
	arr := strings.SplitN(val, ":", 2)
	if len(arr) != 2 || arr[0] == "" || arr[1] == "" {
		return false
	}
	return true
}

// proxyParser provides functions to parse different types of parent proxy
type proxyParser struct{}

func (p proxyParser) ProxySocks5(val string) {
	if err := checkServerAddr(val); err != nil {
		Fatal("parent socks server", err)
	}
	parentProxy.add(newSocksParent(val))
}

func (pp proxyParser) ProxyHttp(val string) {
	var userPasswd, server string

	idx := strings.LastIndex(val, "@")
	if idx == -1 {
		server = val
	} else {
		userPasswd = val[:idx]
		server = val[idx+1:]
	}

	if err := checkServerAddr(server); err != nil {
		Fatal("parent http server", err)
	}

	config.saveReqLine = true

	parent := newHttpParent(server)
	parent.initAuth(userPasswd)
	parentProxy.add(parent)
}

func (pp proxyParser) ProxyHttps(val string) {
	var userPasswd, server string

	idx := strings.LastIndex(val, "@")
	if idx == -1 {
		server = val
	} else {
		userPasswd = val[:idx]
		server = val[idx+1:]
	}

	if err := checkServerAddr(server); err != nil {
		Fatal("parent http server", err)
	}

	config.saveReqLine = true

	parent := newHttpsParent(server)
	parent.initAuth(userPasswd)
	parentProxy.add(parent)
}

// Parse method:passwd@server:port
func parseMethodPasswdServer(val string) (method, passwd, server string, err error) {
	// Use the right-most @ symbol to seperate method:passwd and server:port.
	idx := strings.LastIndex(val, "@")
	if idx == -1 {
		err = errors.New("requires both encrypt method and password")
		return
	}

	methodPasswd := val[:idx]
	server = val[idx+1:]
	if err = checkServerAddr(server); err != nil {
		return
	}

	// Password can have : inside, but I don't recommend this.
	arr := strings.SplitN(methodPasswd, ":", 2)
	if len(arr) != 2 {
		err = errors.New("method and password should be separated by :")
		return
	}
	method = arr[0]
	passwd = arr[1]
	return
}

// parse shadowsocks proxy
func (pp proxyParser) ProxySs(val string) {
	method, passwd, server, err := parseMethodPasswdServer(val)
	if err != nil {
		Fatal("shadowsocks parent", err)
	}
	parent := newShadowsocksParent(server)
	parent.initCipher(method, passwd)
	parentProxy.add(parent)
}

func (pp proxyParser) ProxyMeow(val string) {
	method, passwd, server, err := parseMethodPasswdServer(val)
	if err != nil {
		Fatal("meow parent", err)
	}

	if err := checkServerAddr(server); err != nil {
		Fatal("parent meow server", err)
	}

	config.saveReqLine = true
	parent := newMeowParent(server, method, passwd)
	parentProxy.add(parent)
}

// listenParser provides functions to parse different types of listen addresses
type listenParser struct{}

func (lp listenParser) ListenHttp(val string, proto string) {
	if cmdHasListenAddr {
		return
	}

	arr := strings.Fields(val)
	if len(arr) > 2 {
		Fatal("too many fields in listen =", proto, val)
	}

	var addr, addrInPAC string
	addr = arr[0]
	if len(arr) == 2 {
		addrInPAC = arr[1]
	}

	if err := checkServerAddr(addr); err != nil {
		Fatal("listen", proto, "server", err)
	}
	addListenProxy(newHttpProxy(addr, addrInPAC, proto))
}

func (lp listenParser) ListenMeow(val string) {
	if cmdHasListenAddr {
		return
	}
	method, passwd, addr, err := parseMethodPasswdServer(val)
	if err != nil {
		Fatal("listen meow", err)
	}
	addListenProxy(newMeowProxy(method, passwd, addr))
}

// configParser provides functions to parse options in config file.
type configParser struct{}

func (p configParser) ParseProxy(val string) {
	parser := reflect.ValueOf(proxyParser{})
	zeroMethod := reflect.Value{}

	arr := strings.Split(val, "://")
	if len(arr) != 2 {
		Fatal("proxy has no protocol specified:", val)
	}
	protocol := arr[0]

	methodName := "Proxy" + strings.ToUpper(protocol[0:1]) + protocol[1:]
	method := parser.MethodByName(methodName)
	if method == zeroMethod {
		Fatalf("no such protocol \"%s\"\n", arr[0])
	}
	args := []reflect.Value{reflect.ValueOf(arr[1])}
	method.Call(args)
}

func (p configParser) ParseListen(val string) {
	if cmdHasListenAddr {
		return
	}

	parser := reflect.ValueOf(listenParser{})
	zeroMethod := reflect.Value{}

	var protocol, server string
	arr := strings.Split(val, "://")
	if len(arr) == 1 {
		protocol = "http"
		server = val
		configNeedUpgrade = true
	} else {
		protocol = arr[0]
		server = arr[1]
	}

	methodName := "Listen" + strings.ToUpper(protocol[0:1]) + protocol[1:]
	if methodName == "ListenHttps" {
		methodName = "ListenHttp"
	}
	method := parser.MethodByName(methodName)
	if method == zeroMethod {
		Fatalf("no such listen protocol \"%s\"\n", arr[0])
	}
	if methodName == "ListenMeow" {
		method.Call([]reflect.Value{reflect.ValueOf(server)})
	} else {
		method.Call([]reflect.Value{reflect.ValueOf(server), reflect.ValueOf(protocol)})
	}
}

func (p configParser) ParseLogFile(val string) {
	config.LogFile = expandConfigPath(val)
}

func (p configParser) ParseAddrInPAC(val string) {
	configNeedUpgrade = true
	arr := strings.Split(val, ",")
	for i, s := range arr {
		if s == "" {
			continue
		}
		s = strings.TrimSpace(s)
		host, _, err := net.SplitHostPort(s)
		if err != nil {
			Fatal("proxy address in PAC", err)
		}
		if host == "0.0.0.0" {
			Fatal("can't use 0.0.0.0 as proxy address in PAC")
		}
		if hp, ok := listenProxy[i].(*httpProxy); ok {
			hp.addrInPAC = s
		} else {
			Fatal("can't specify address in PAC for non http proxy")
		}
	}
}

func (p configParser) ParseSocksParent(val string) {
	var pp proxyParser
	pp.ProxySocks5(val)
	configNeedUpgrade = true
}

func (p configParser) ParseSshServer(val string) {
	arr := strings.Split(val, ":")
	if len(arr) == 2 {
		val += ":22"
	} else if len(arr) == 3 {
		if arr[2] == "" {
			val += "22"
		}
	} else {
		Fatal("sshServer should be in the form of: user@server:local_socks_port[:server_ssh_port]")
	}
	// add created socks server
	p.ParseSocksParent("127.0.0.1:" + arr[1])
	config.SshServer = append(config.SshServer, val)
}

var http struct {
	parent    *httpParent
	serverCnt int
	passwdCnt int
}

func (p configParser) ParseHttpParent(val string) {
	if err := checkServerAddr(val); err != nil {
		Fatal("parent http server", err)
	}
	config.saveReqLine = true
	http.parent = newHttpParent(val)
	parentProxy.add(http.parent)
	http.serverCnt++
	configNeedUpgrade = true
}

func (p configParser) ParseHttpUserPasswd(val string) {
	if !isUserPasswdValid(val) {
		Fatal("httpUserPassword syntax wrong, should be in the form of user:passwd")
	}
	if http.passwdCnt >= http.serverCnt {
		Fatal("must specify httpParent before corresponding httpUserPasswd")
	}
	http.parent.initAuth(val)
	http.passwdCnt++
}

func (p configParser) ParseLoadBalance(val string) {
	switch val {
	case "backup":
		config.LoadBalance = loadBalanceBackup
	case "hash":
		config.LoadBalance = loadBalanceHash
	case "latency":
		config.LoadBalance = loadBalanceLatency
	default:
		Fatalf("invalid loadBalance mode: %s\n", val)
	}
}

func (p configParser) ParseProxyMode(val string) {
	switch strings.ToLower(val) {
	case "default", "":
		config.ProxyMode = proxyModeDefault
	case "keep":
		config.ProxyMode = proxyModeKeep
	case "cow":
		config.ProxyMode = proxyModeCow
	default:
		Fatalf("invalid proxyMode: %s, should be default, keep or cow\n", val)
	}
}

func (p configParser) ParseDirectFile(val string) {
	config.DirectFile = expandConfigPath(val)
	ensureDomainListFile(config.DirectFile, "# 白名单：命中后直连。支持 IP、CIDR、IP 范围、域名、通配符、URL path/query 片段。"+newLine)
}

func (p configParser) ParseProxyFile(val string) {
	config.ProxyFile = expandConfigPath(val)
	ensureDomainListFile(config.ProxyFile, "# 强制代理名单：命中后使用二级代理。支持 IP、CIDR、IP 范围、域名、通配符、URL path/query 片段。"+newLine)
}

func (p configParser) ParseRejectFile(val string) {
	config.RejectFile = expandConfigPath(val)
	ensureDomainListFile(config.RejectFile, "# 黑名单：命中后返回 MEOW 自带拦截页。支持 IP、CIDR、IP 范围、域名、通配符、URL path/query 片段。"+newLine)
}

var shadow struct {
	parent *shadowsocksParent
	passwd string
	method string

	serverCnt int
	passwdCnt int
	methodCnt int
}

func (p configParser) ParseShadowSocks(val string) {
	if shadow.serverCnt-shadow.passwdCnt > 1 {
		Fatal("must specify shadowPasswd for every shadowSocks server")
	}
	// create new shadowsocks parent if both server and password are given
	// previously
	if shadow.parent != nil && shadow.serverCnt == shadow.passwdCnt {
		if shadow.methodCnt < shadow.serverCnt {
			shadow.method = ""
			shadow.methodCnt = shadow.serverCnt
		}
		shadow.parent.initCipher(shadow.method, shadow.passwd)
	}
	if val == "" { // the final call
		shadow.parent = nil
		return
	}
	if err := checkServerAddr(val); err != nil {
		Fatal("shadowsocks server", err)
	}
	shadow.parent = newShadowsocksParent(val)
	parentProxy.add(shadow.parent)
	shadow.serverCnt++
	configNeedUpgrade = true
}

func (p configParser) ParseShadowPasswd(val string) {
	if shadow.passwdCnt >= shadow.serverCnt {
		Fatal("must specify shadowSocks before corresponding shadowPasswd")
	}
	if shadow.passwdCnt+1 != shadow.serverCnt {
		Fatal("must specify shadowPasswd for every shadowSocks")
	}
	shadow.passwd = val
	shadow.passwdCnt++
}

func (p configParser) ParseShadowMethod(val string) {
	if shadow.methodCnt >= shadow.serverCnt {
		Fatal("must specify shadowSocks before corresponding shadowMethod")
	}
	// shadowMethod is optional
	shadow.method = val
	shadow.methodCnt++
}

func checkShadowsocks() {
	if shadow.serverCnt != shadow.passwdCnt {
		Fatal("number of shadowsocks server and password does not match")
	}
	// parse the last shadowSocks option again to initialize the last
	// shadowsocks server
	parser := configParser{}
	parser.ParseShadowSocks("")
}

// Put actual authentication related config parsing in auth.go, so config.go
// doesn't need to know the details of authentication implementation.

func (p configParser) ParseUserPasswd(val string) {
	config.UserPasswd = val
	if !isUserPasswdValid(config.UserPasswd) {
		Fatal("userPassword syntax wrong, should be in the form of user:passwd")
	}
}

func (p configParser) ParseUserPasswdFile(val string) {
	userPasswdFile := expandConfigPath(val)
	err := isFileExists(userPasswdFile)
	if err != nil {
		Fatal("userPasswdFile:", err)
	}
	config.UserPasswdFile = userPasswdFile
}

func (p configParser) ParseAllowedClient(val string) {
	config.AllowedClient = val
}

func (p configParser) ParseAuthTimeout(val string) {
	config.AuthTimeout = parseDuration(val, "authTimeout")
}

func (p configParser) ParseCore(val string) {
	config.Core = parseInt(val, "core")
}

func (p configParser) ParseHttpErrorCode(val string) {
	config.HttpErrorCode = parseInt(val, "httpErrorCode")
}

func (p configParser) ParseParentFailureFeedback(val string) {
	config.ParentFailureFeedback = parseBool(val, "parentFailureFeedback")
}

func (p configParser) ParseParentProbeFailStatus(val string) {
	status := make(map[int]bool)
	for _, s := range strings.Split(val, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		code := parseInt(s, "parentProbeFailStatus")
		if code < 100 || code > 999 {
			Fatalf("parentProbeFailStatus invalid status code: %d\n", code)
		}
		status[code] = true
	}
	config.ParentProbeFailStatus = status
}

func (p configParser) ParseParentProbeURL(val string) {
	val = strings.TrimSpace(val)
	if val == "" {
		probeURL, _ := newParentProbeURL(defaultParentProbeURL)
		parentProbeURL = probeURL
		config.ParentProbeURL = probeURL.HostPort
		fmt.Printf("parentProbeURL 为空，已使用默认值 %s\n", config.ParentProbeURL)
		return
	}
	probeURL, err := newParentProbeURL(val)
	if err != nil {
		Fatal("parentProbeURL:", err)
	}
	parentProbeURL = probeURL
	config.ParentProbeURL = probeURL.HostPort
}

func (p configParser) ParseParentProbeInterval(val string) {
	interval := parseDuration(val, "parentProbeInterval")
	if interval < minParentProbeInterval {
		fmt.Printf("parentProbeInterval %s 小于最小值 %s，已使用默认值 %s\n",
			interval, minParentProbeInterval, defaultParentProbeInterval)
		config.ParentProbeInterval = defaultParentProbeInterval
		return
	}
	config.ParentProbeInterval = interval
}

func (p configParser) ParseReadTimeout(val string) {
	config.ReadTimeout = parseDuration(val, "readTimeout")
}

func (p configParser) ParseDialTimeout(val string) {
	config.DialTimeout = parseDuration(val, "dialTimeout")
}

func (p configParser) ParseJudgeByIP(val string) {
	config.JudgeByIP = parseBool(val, "judgeByIP")
}

func (p configParser) ParseQQWryFile(val string) {
	config.QQWryFile = expandConfigPath(val)
}

func (p configParser) ParseQQWryUpdateURL(val string) {
	config.QQWryUpdateURL = val
}

func (p configParser) ParseQQWryUpdateInterval(val string) {
	config.QQWryUpdateInterval = parseDuration(val, "qqwryUpdateInterval")
}

func (p configParser) ParseCert(val string) {
	config.Cert = expandConfigPath(val)
}

func (p configParser) ParseKey(val string) {
	config.Key = expandConfigPath(val)
}

type configOptionTemplate struct {
	Key  string
	Body string
}

func defaultConfigOptions() []configOptionTemplate {
	return []configOptionTemplate{
		{"listen", "#############################\n# 监听地址，设为 0.0.0.0 可以监听所有网卡并共享给局域网使用\n#############################\nlisten = http://" + defaultListenAddr + "\n"},
		{"judgeByIP", "#############################\n# 通过 IP 判断是否直连，默认开启\n#############################\n#judgeByIP = true\n"},
		{"qqwryFile", "#############################\n# QQWry.dat 本地 IP 库路径；相对路径按 rc 文件所在目录解析\n# 仅用于 IPv4，读取失败时自动回退到内置中国 IP 库\n#############################\n#qqwryFile = " + config.QQWryFile + "\n"},
		{"qqwryUpdateURL", "#############################\n# QQWry.dat 在线更新地址；默认使用 FW27623/qqwry 的最新数据直链\n#############################\n#qqwryUpdateURL = " + config.QQWryUpdateURL + "\n"},
		{"qqwryUpdateInterval", "#############################\n# QQWry.dat 自动更新频率；设置为 0s 可关闭定时更新\n# 示例：24h 表示每 24 小时检查并下载一次\n#############################\n#qqwryUpdateInterval = 24h\n"},
		{"proxyMode", "#############################\n# 代理模式，可选 default、keep、cow\n# default：保持 MEOW 当前白名单模式不变\n# keep：在 default 基础上，上游代理全部失败时尝试直连兜底\n# cow：默认直连，直连失败时快速改用上游代理尝试连接\n#############################\n#proxyMode = default\n"},
		{"logFile", "#############################\n# 日志文件路径，如不指定则输出到 stdout；相对路径按 rc 文件所在目录解析\n#############################\n#logFile = meow.log\n"},
		{"loadBalance", "#############################\n# 多个二级代理时的负载均衡策略，可选 backup、hash、latency\n#############################\n#loadBalance = backup\n"},
		{"proxy", "#############################\n# 指定二级代理，可重复配置\n# 示例：proxy = socks5://127.0.0.1:1080\n# 示例：proxy = http://user:password@127.0.0.1:8080\n# 示例：proxy = ss://aes-256-cfb:password@1.2.3.4:8388\n#############################\n#proxy = socks5://127.0.0.1:1080\n"},
		{"sshServer", "#############################\n# 执行 ssh 命令创建 SOCKS5 代理，需要系统已有 ssh 命令和公钥认证\n#############################\n#sshServer = user@server:local_socks_port[:server_ssh_port]\n"},
		{"allowedClient", "#############################\n# 允许免认证访问的客户端 IP 或网段，多个项用逗号分隔\n#############################\n#allowedClient = 127.0.0.1, 192.168.1.0/24\n"},
		{"userPasswd", "#############################\n# 要求客户端通过用户名密码认证\n#############################\n#userPasswd = username:password\n"},
		{"userPasswdFile", "#############################\n# 从文件读取多个用户名密码，文件每行格式：username:password[:port]\n# 相对路径按 rc 文件所在目录解析\n#############################\n#userPasswdFile = user_passwd.txt\n"},
		{"authTimeout", "#############################\n# 认证失效时间，语法示例：2h、30m、2h30m\n#############################\n#authTimeout = 2h\n"},
		{"httpErrorCode", "#############################\n# 将指定 HTTP error code 认为是被干扰并使用二级代理重试\n#############################\n#httpErrorCode = 403\n"},
		{"parentFailureFeedback", "#############################\n# 请求阶段发生读写、超时、连接重置等错误时，将当前二级代理标记为失败\n#############################\n#parentFailureFeedback = true\n"},
		{"parentProbeFailStatus", "#############################\n# 二级代理 CONNECT 探测时，指定哪些响应码视为代理不可用，多个状态码用逗号分隔\n#############################\n#parentProbeFailStatus = 403,407,502,503,504\n"},
		{"parentProbeURL", "#############################\n# 二级代理连通性/延迟探测地址，仅 loadBalance = latency 时使用\n# 格式必须为 host:port；为空时使用默认值 " + defaultParentProbeURL + "\n# 域名示例：www.google.com:443；IPv4 示例：1.1.1.1:443；IPv6 示例：[2001:4860:4860::8888]:443\n#############################\n#parentProbeURL = " + config.ParentProbeURL + "\n"},
		{"parentProbeInterval", "#############################\n# 二级代理连通性/延迟探测周期，仅 loadBalance = latency 时使用\n# 默认 60s；最小 10s，小于 10s 时会忽略配置并使用默认值，防止探测过于频繁\n#############################\n#parentProbeInterval = 60s\n"},
		{"core", "#############################\n# 最多允许使用多少个 CPU 核\n#############################\n#core = 2\n"},
		{"readTimeout", "#############################\n# 读取超时时间\n#############################\n#readTimeout = 2m\n"},
		{"dialTimeout", "#############################\n# 连接超时时间\n#############################\n#dialTimeout = 30s\n"},
		{"directFile", "#############################\n# 自定义白名单文件路径；相对路径按 rc 文件所在目录解析\n#############################\n#directFile = " + config.DirectFile + "\n"},
		{"proxyFile", "#############################\n# 自定义强制代理名单文件路径；相对路径按 rc 文件所在目录解析\n#############################\n#proxyFile = " + config.ProxyFile + "\n"},
		{"rejectFile", "#############################\n# 自定义黑名单文件路径；相对路径按 rc 文件所在目录解析\n# 支持具体 IP、CIDR、IP 范围、具体域名、二级域名、通配符、URL path/query 片段\n#############################\n#rejectFile = " + config.RejectFile + "\n"},
		{"cert", "#############################\n# HTTPS 本地代理证书路径，使用 https listen 时需要；相对路径按 rc 文件所在目录解析\n#############################\n#cert = cert.pem\n"},
		{"key", "#############################\n# HTTPS 本地代理私钥路径，使用 https listen 时需要；相对路径按 rc 文件所在目录解析\n#############################\n#key = key.pem\n"},
	}
}

func ensureConfigFiles(rc string) {
	if err := os.MkdirAll(filepath.Dir(rc), 0755); err != nil {
		Fatal("can't create config directory:", err)
	}

	created := false
	if _, err := os.Stat(rc); os.IsNotExist(err) {
		if err := writeDefaultConfig(rc); err != nil {
			Fatal("can't create default config file:", err)
		}
		fmt.Println("未找到配置文件，已自动创建默认配置:", rc)
		created = true
	} else if err != nil {
		Fatal("fail to get config file:", err)
	}

	ensureDomainListFile(config.DirectFile, "# 白名单：命中后直连。支持 IP、CIDR、IP 范围、域名、通配符、URL path/query 片段。"+newLine)
	ensureDomainListFile(config.ProxyFile, "# 强制代理名单：命中后使用二级代理。支持 IP、CIDR、IP 范围、域名、通配符、URL path/query 片段。"+newLine)
	ensureDomainListFile(config.RejectFile, "# 黑名单：命中后返回 MEOW 自带拦截页。支持 IP、CIDR、IP 范围、域名、通配符、URL path/query 片段。"+newLine+"# 示例：*.ads.example.com"+newLine+"# 示例：/ad/"+newLine+"# 示例：?ad.js"+newLine)

	if !created {
		appendMissingConfigOptions(rc)
	}
}

func writeDefaultConfig(rc string) error {
	f, err := os.Create(rc)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.WriteString(f, "# MEOW 配置文件。程序自动生成，按需取消注释并修改配置项。"+newLine+newLine)
	if err != nil {
		return err
	}
	for _, opt := range defaultConfigOptions() {
		if _, err = io.WriteString(f, strings.ReplaceAll(opt.Body, "\n", newLine)+newLine); err != nil {
			return err
		}
	}
	return nil
}

func ensureDomainListFile(file, header string) {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		if err := os.WriteFile(file, []byte(header), 0644); err != nil {
			Fatal("can't create list file:", file, err)
		}
		fmt.Println("已自动创建名单文件:", file)
	} else if err != nil {
		Fatal("fail to get list file:", file, err)
	}
}

func appendMissingConfigOptions(rc string) {
	data, err := os.ReadFile(rc)
	if err != nil {
		Fatal("Error opening config file:", err)
	}
	exists := make(map[string]bool)
	lines := strings.Split(string(data), "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		line = strings.TrimPrefix(line, "\ufeff")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		v := strings.SplitN(line, "=", 2)
		if len(v) == 2 {
			exists[strings.TrimSpace(v[0])] = true
		}
	}

	var missing []configOptionTemplate
	for _, opt := range defaultConfigOptions() {
		if !exists[opt.Key] {
			missing = append(missing, opt)
		}
	}
	if len(missing) == 0 {
		return
	}

	f, err := os.OpenFile(rc, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		Fatal("can't append config file:", err)
	}
	defer f.Close()

	if _, err = io.WriteString(f, newLine+"#############################"+newLine+"# 以下配置项由当前版本自动补全，请按需取消注释并修改"+newLine+"#############################"+newLine); err != nil {
		Fatal("can't append config file:", err)
	}
	for _, opt := range missing {
		if _, err = io.WriteString(f, strings.ReplaceAll(opt.Body, "\n", newLine)+newLine); err != nil {
			Fatal("can't append config file:", err)
		}
	}
	fmt.Printf("检测到配置文件缺少 %d 个配置项，已自动补全说明: %s\n", len(missing), rc)
}

// overrideConfig should contain options from command line to override options
// in config file.
func parseConfig(rc string, override *Config) {
	// fmt.Println("rcFile:", path)
	f, err := os.Open(expandTilde(rc))
	if err != nil {
		Fatal("Error opening config file:", err)
	}

	IgnoreUTF8BOM(f)

	scanner := bufio.NewScanner(f)

	parser := reflect.ValueOf(configParser{})
	zeroMethod := reflect.Value{}
	var lines []string // store lines for upgrade

	var n int
	for scanner.Scan() {
		lines = append(lines, scanner.Text())

		n++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}

		v := strings.SplitN(line, "=", 2)
		if len(v) != 2 {
			Fatal("config syntax error on line", n)
		}
		key, val := strings.TrimSpace(v[0]), strings.TrimSpace(v[1])

		methodName := "Parse" + strings.ToUpper(key[0:1]) + key[1:]
		method := parser.MethodByName(methodName)
		if method == zeroMethod {
			Fatalf("no such option \"%s\" on line %d\n", key, n)
		}
		if val == "" && !allowEmptyConfigValue(key) {
			Fatalf("empty %s on line %d, please comment or remove unused option\n", key, n)
		}
		args := []reflect.Value{reflect.ValueOf(val)}
		method.Call(args)
	}
	if scanner.Err() != nil {
		Fatalf("Error reading rc file: %v\n", scanner.Err())
	}
	f.Close()

	overrideConfig(&config, override)
	checkConfig()

	if configNeedUpgrade {
		upgradeConfig(rc, lines)
	}
}

func upgradeConfig(rc string, lines []string) {
	newrc := rc + ".upgrade"
	f, err := os.Create(newrc)
	if err != nil {
		fmt.Println("can't create upgraded config file")
		return
	}

	// Upgrade config.
	proxyId := 0
	listenId := 0
	w := bufio.NewWriter(f)
	for _, line := range lines {
		line := strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			w.WriteString(line + newLine)
			continue
		}

		v := strings.Split(line, "=")
		key := strings.TrimSpace(v[0])

		switch key {
		case "listen":
			listen := listenProxy[listenId]
			listenId++
			w.WriteString(listen.genConfig() + newLine)
			// comment out original
			w.WriteString("#" + line + newLine)
		case "httpParent", "shadowSocks", "socksParent":
			backPool, ok := parentProxy.(*backupParentPool)
			if !ok {
				panic("initial parent pool should be backup pool")
			}
			parent := backPool.parent[proxyId]
			proxyId++
			w.WriteString(parent.genConfig() + newLine)
			// comment out original
			w.WriteString("#" + line + newLine)
		case "httpUserPasswd", "shadowPasswd", "shadowMethod", "addrInPAC":
			// just comment out
			w.WriteString("#" + line + newLine)
		case "proxy":
			proxyId++
			w.WriteString(line + newLine)
		default:
			w.WriteString(line + newLine)
		}
	}
	w.Flush()
	f.Close() // Must close file before renaming, otherwise will fail on windows.

	// Rename new and old config file.
	if err := os.Rename(rc, rc+"0.8"); err != nil {
		fmt.Println("can't backup config file for upgrade:", err)
		return
	}
	if err := os.Rename(newrc, rc); err != nil {
		fmt.Println("can't rename upgraded rc to original name:", err)
		return
	}
}

func overrideConfig(oldconfig, override *Config) {
	newVal := reflect.ValueOf(override).Elem()
	oldVal := reflect.ValueOf(oldconfig).Elem()

	// typeOfT := newVal.Type()
	for i := 0; i < newVal.NumField(); i++ {
		newField := newVal.Field(i)
		oldField := oldVal.Field(i)
		// log.Printf("%d: %s %s = %v\n", i,
		// typeOfT.Field(i).Name, newField.Type(), newField.Interface())
		switch newField.Kind() {
		case reflect.String:
			s := newField.String()
			if s != "" {
				oldField.SetString(s)
			}
		case reflect.Int:
			i := newField.Int()
			if i != 0 {
				oldField.SetInt(i)
			}
		}
	}
}

// Must call checkConfig before using config.
func checkConfig() {
	checkShadowsocks()
	// listenAddr must be handled first, as addrInPAC dependends on this.
	if listenProxy == nil {
		listenProxy = []Proxy{newHttpProxy(defaultListenAddr, "", "http")}
	}
}
