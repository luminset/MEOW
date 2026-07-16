# MEOW Proxy

当前版本：1.5-nohafix 修改版（源版本 1.5） [CHANGELOG](CHANGELOG.md)
[![Build Status](https://travis-ci.org/netheril96/MEOW.png?branch=master)](https://travis-ci.org/netheril96/MEOW)

<pre>
       /\
   )  ( ')     MEOW 是 [COW](https://github.com/cyfdecyf/cow) 的一个派生版本
  (  /  )      MEOW 与 COW 最大的不同之处在于，COW 采用黑名单模式， 而 MEOW 采用白名单模式
   \(__)|      国内网站直接连接，其他的网站使用代理连接
</pre>


## NOHAFIX 修改版说明

本修改版保留源版本号 `1.5`，运行版本格式为 `1.5-nohafix[构建次数]`。使用 `script/build-nohafix.ps1` 构建时会自动递增 `build_count.txt`，并生成 Windows amd64、Linux ARMv7、Linux ARM64 三个目标产物。

### 构建与兼容性

- 增加 `go.mod` 与 `go.sum`，使旧 GOPATH 项目可在现代 Go 模块模式下构建。
- 修复 `chinaip_gen.go` 文件头异常字符导致的编译失败。
- 修复 `chinaip_init.go` 与 `chinaip_data.go` 中 IPv6 数据重复声明导致的编译失败。
- 修复新版 Go 对测试函数命名的检查问题。
- 当前项目可在 Windows 下本机构建，并可通过 Go 原生交叉编译输出 Linux ARMv7 与 Linux ARM64 程序。

### 配置文件自愈

- Windows 下首次运行如果缺少 `rc.txt`，程序会自动创建默认配置文件。
- 程序会自动创建 `direct.txt`、`proxy.txt`、`reject.txt` 名单文件，并在控制台输出提示信息。
- 如果已有旧版本配置文件，但缺少当前版本支持的配置项，程序会在文件末尾自动追加缺失配置项的注释说明与示例，不覆盖用户已有配置。
- 如果配置中指定了自定义 `directFile`、`proxyFile`、`rejectFile`，但对应文件不存在，程序会自动创建。
- 配置中的文件路径类选项支持相对路径，相对路径统一按 `rc` 文件所在目录解析，包括 `logFile`、`directFile`、`proxyFile`、`rejectFile`、`userPasswdFile`、`qqwryFile`、`cert`、`key`。
- 修复 Windows UTF-8 BOM 导致第一行配置项无法被识别的问题。

### IP 库增强

- 保留原有内置中国 IP CIDR 库，原始生成地址仍为 `17mon/china_ip_list` 的 IPv4 与 IPv6 列表。
- 新增 `QQWry.dat` 本地 IPv4 库支持；当本地 `QQWry.dat` 不存在或读取失败时，程序会立即回退到内置中国 IP 库，并在后台尝试下载更新。
- 如果本地 `QQWry.dat` 可读，且比本地 `china_ip_list` 文件更新，则 IPv4 判断优先使用 `QQWry.dat`；如果 `QQWry.dat` 无法判断，则继续回退到内置/本地 CIDR 库。
- IPv6 仍使用原有内置/本地 CIDR 库。
- `QQWry.dat` 默认更新地址为 `https://raw.githubusercontent.com/FW27623/qqwry/main/qqwry.dat`，可通过配置改为其他镜像。

配置示例：

```ini
# QQWry.dat 本地 IP 库路径；相对路径按 rc 文件所在目录解析
#qqwryFile = QQWry.dat

# QQWry.dat 在线更新地址
#qqwryUpdateURL = https://raw.githubusercontent.com/FW27623/qqwry/main/qqwry.dat

# QQWry.dat 自动更新频率；设置为 0s 可关闭定时更新
#qqwryUpdateInterval = 24h
```

### 黑白名单增强

`direct.txt`、`proxy.txt`、`reject.txt` 均支持以下规则：

- 具体 IP：`203.0.113.8`
- CIDR IP 段：`198.51.100.0/24`
- IP 范围：`192.0.2.10-192.0.2.20`
- 具体域名：`www.example.com`
- 二级域名：`example.com`
- 域名通配符：`*.ads.example.net`
- URL path 片段：`/ad/`
- URL query 片段：`?ad.js`

命中 `reject.txt` 时，程序会返回自带的 `403 Forbidden` 拦截页，提示访问的指定地址已经被拦截，并展示被拦截的目标地址。

### 代理模式

新增配置项 `proxyMode`，可写入 `rc.txt`：

```ini
# 代理模式，可选 default、keep、cow
# default：保持 MEOW 当前白名单模式不变
# keep：在 default 基础上，上游代理全部失败时尝试直连兜底
# cow：默认直连，直连失败时快速改用上游代理尝试连接
proxyMode = default
```

三种模式说明：

- `default`：保持当前 MEOW 白名单模式不变。国内或白名单地址直连，其他地址走上游代理。
- `keep`：保连接模式。默认判定逻辑不变；当请求需要走上游代理但所有上游代理连接失败时，程序会再尝试直接连接目标网站。
- `cow`：COW 模式。未命中黑名单或显式代理规则时默认直连；如果直连失败且配置了上游代理，则快速切换为通过上游代理尝试连接。

## MEOW 可以用来
- 作为全局 HTTP 代理（支持 PAC），可以智能分流（直连国内网站、使用代理连接其他网站）
- 将 SOCKS5 等代理转换为 HTTP 代理，HTTP 代理能最大程度兼容各种软件（可以设置为程序代理）和设备（设置为系统全局代理）
- 架设在内网（或者公网），为其他设备提供智能分流代理
- 编译成一个无需任何依赖的可执行文件运行，支持各种平台（Win / Linux / OS X），甚至是树莓派（Linux ARM）

## 获取

- **从源码安装:** 安装 [Go](http://golang.org/doc/install)，然后 `go get github.com/netheril96/MEOW`

## 配置

编辑 `~/.meow/rc` (OS X, Linux) 或 `rc.txt` (Windows)，例子：

    # 监听地址，设为0.0.0.0可以监听所有端口，共享给局域网使用
    listen = http://127.0.0.1:4411
    # 至少指定一个上级代理
    # SOCKS5 上级代理
    # proxy = socks5://127.0.0.1:1080
    # HTTP 上级代理
    # proxy = http://127.0.0.1:8087
    # shadowsocks 上级代理
    # proxy = ss://aes-128-cfb:password@example.server.com:25
    # HTTPS 上级代理
    # proxy = https://user:password@example.server.com:port

## 工作方式

当 MEOW 启动时会从配置文件加载直连列表和强制使用代理列表，详见下面两节。

当通过 MEOW 访问一个网站时，MEOW 会：

- 检查域名是否在直连列表中，如果在则直连
- 检查域名是否在强制使用代理列表中，如果在则通过代理连接
- **检查域名的 IP 是否为国内 IP**
    - 通过本地 DNS 解析域名，得到域名的 IP
    - 如果是国内 IP 则直连，否则通过代理连接
    - 将域名加入临时的直连或者强制使用代理列表，下次可以不用 DNS 解析直接判断域名是否直连

## 直连列表

直接连接的域名列表保存在 `~/.meow/direct` (OS X, Linux) 或 `direct.txt` (Windows)

匹配域名**按 . 分隔的后两部分**或者**整个域名**，例子：

-  `baidu.com` => `*.baidu.com`
-  `com.cn` => `*.com.cn`
-  `edu.cn` => `*.edu.cn`
-  `music.163.com` => `music.163.com`

一般是**确定**要直接连接的网站

## 强制使用代理列表

强制使用代理连接的域名列表保存在 `~/.meow/proxy` (OS X, Linux) 或 `proxy.txt` (Windows)，语法格式与直连列表相同
（注意：匹配的是域名**按 . 分隔的后两部分**或者**整个域名**）

## 致谢

- @cyfdecyf - COW author
- @renzhn - Original MEOW author
- Github - Github Student Pack
- https://www.pandafan.org/pac/index.html - Domain White List
- https://github.com/Leask/Flora_Pac - CN IP Data
- https://github.com/17mon/china_ip_list - CN IP Data
