## 更新说明
- 2026-07-16 Version 1.5-nohafix6

       * 新增 Go Modules 支持，补充 `go.mod` 与 `go.sum`
       * 修复现代 Go 环境下的编译问题，包括 `chinaip_gen.go` 文件头异常字符、IPv6 数据重复声明、测试函数命名问题
       * 成功支持 Windows amd64 本机构建，以及 Linux ARMv7、Linux ARM64 交叉编译输出
       * Windows 下缺少 `rc.txt` 时不再直接退出，改为自动创建默认配置文件
       * 自动创建 `direct.txt`、`proxy.txt`、`reject.txt` 名单文件，并输出提示信息
       * 旧配置文件缺少新配置项时，自动追加缺失配置项及说明，不覆盖用户已有配置
       * 修复 Windows UTF-8 BOM 导致第一行配置项无法识别的问题
       * 自定义 `directFile`、`proxyFile`、`rejectFile` 指向的名单文件不存在时自动创建
       * 增强 `direct`、`proxy`、`reject` 名单规则，支持具体 IP、CIDR、IP 范围、具体域名、二级域名、域名通配符、URL path/query 片段匹配
       * 命中 `reject` 黑名单时返回程序自带 `403 Forbidden` 拦截页，提示访问地址已被拦截
       * 新增 `proxyMode` 配置项，支持 `default`、`keep`、`cow` 三种代理模式
       * `default` 模式保持原 MEOW 白名单模式不变
       * `keep` 保连接模式在所有上游代理失败后尝试直连目标网站
       * `cow` 模式默认直连，直连失败后快速切换为上游代理尝试连接
       * 新增 `QQWry.dat` 本地 IPv4 IP 库支持，读取失败时自动回退到内置中国 IP 库
       * 新增 `qqwryFile`、`qqwryUpdateURL`、`qqwryUpdateInterval` 配置项，支持按频率后台更新本地 `QQWry.dat`
       * `QQWry.dat` 缺失或损坏时不阻塞程序启动，后台尝试更新，最低回退到内置 IP 库
       * 修复文件路径类配置项的相对路径解析，确保 `logFile`、`directFile`、`proxyFile`、`rejectFile`、`userPasswdFile`、`qqwryFile`、`cert`、`key` 均按 `rc` 文件所在目录读取
       * 新增 `parentProbeURL` 与 `parentProbeInterval` 配置项，仅在 `loadBalance = latency` 时用于上游代理连通性和延迟探测
       * `parentProbeInterval` 增加最小值保护，小于 10s 时自动回退到默认 60s，避免探测过于频繁
       * `parentProbeURL` 支持域名、IPv4、IPv6 的 `host:port` 写法；空值视为非关键错误并回退到默认探测地址
       * 配置文件空值和未知配置项错误增加行号提示，便于定位错误位置

- 2016-09-29 Version 1.5

       * 更新中国IP列表

- 2016-02-18 Version 1.3.4

       * 使用 Go 1.6 编译，请重新下载
       
- 2015-12-03 Version 1.3.4

       * 修正客户端连接未正确关闭 bug
       * 修正对文件描述符过多错误的判断（too many open files）

- 2015-11-22 Version 1.3.3

       * 增加 `reject` 拒绝连接列表
       * 支持作为 HTTPS 代理服务器监听
       * 支持 HTTPS 代理服务器作为父代理
	
	
- 2015-10-09 Version 1.3.2

       * 完全托管在 github，不再使用 meowproxy.me 域名，[新的下载地址](https://github.com/renzhn/MEOW/tree/gh-pages/dist/)

- 2015-08-23 Version 1.3.1

       * 去除了端口限制
       * 使用最新的 Go 1.5 编译

- 2015-07-16 Version 1.3

       更新了默认的直连列表、加入了强制使用代理列表，强烈推荐旧版本用户更新 [direct](https://raw.githubusercontent.com/renzhn/MEOW/master/doc/sample-config/direct) 文件和下载 [proxy](https://raw.githubusercontent.com/renzhn/MEOW/master/doc/sample-config/proxy) 文件（或者重新安装）
