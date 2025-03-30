package root

import (
	"fmt"
	"time"

	"github.com/sohaha/zlsgo/zutil"
)

var ExampleConfig = `# zzz 主配置
# 核心配置
core:
    # 配置版本号，勿手动修改
    version: %v
    # 版本检测时间，勿手动修改
    detection_time: %v

# 其他配置
other:
    # 显示毒鸡汤
    du: true
`

var ExampleStressConfig = `# zzz stress%v 配置

# 请求次数
Count: 10
Concurrency: 5
Quiet: false
Compress: true
UserAgent: zzz-stress
Timeout: 10s
DNSPrefetch: true
Headers: "Accept-Encoding:gzip"

# 请求链接列表
Targets:
  # 普通链接
  - URL: https://www.qq.com/
  # 正则链接
  #- URL: https://wx\.qq\.com/api/user/[0-9]{1,4}
  #  RegexURL: true
  # 其他选项
  #- URL: https://qq.com
  #  Method: POST
  #  Compress: false
  #  Body: "{\"username\": \"newuser1\", \"email\": \"newuser1@domain.com\"}"
  #  Headers: "Accept-Encoding:gzip, Content-Type:application/json"
  #  Cookies: "data=123; session=456"
  #  UserAgent: "zzz"
  #  Count: 1

`

var ExampleWatchConfig = `# zzz watch 配置 https://github.com/sohaha/zzz
core:
  # 配置版本号，勿手动修改
  version: %v

# 监控配置
monitor:
  # 使用轮询，开启可以监听挂载目录
  poll: false

  # 要监听的目录，支持通配符*，如“.,*”表示监听当前目录及其所有子目录
  includeDirs:
    - '.,*'

  # 忽略监听的目录
  exceptDirs:
    - '*/.idea/*'
    - '*/.vscode/*'
    - '*/vendor/*'
    - '*/tmp/*'
    - '*/.git/*'
    - '*/.venv/*'
    - '*/node_modules/*'
    - '*/__pycache__/*'
    - '*/target/*'

  # 监听文件的格式，支持通配符*，如“.*”表示监听全部格式文件
  types:
    - .go
    - .php

# 命令
command:
  # 开启监听的同时会后台执行的命令，可以放置一些初始化命令
  # startupExec:
  #  - go version

  # 监听的文件有更改会执行的命令，不支持复杂的命令，如需要请写成脚本调用
  # 支持变量占位符,{{file}} {{ext}} {{changed}}
  # 支持不同平台执行不同命令，如 Windows 下才执行 dir：win@dir，或者 Linux 下：linux@ls -a
  exec:
    - go build -o %s
    - %s

  # 自定义不同类型文件执行命令
  # 上面的 exec 是属于全部文件通用的，如果想不同文件更新执行不同指令可以用 exec+文件后缀（首字母大写） 来设置，如：
  # execGo:
  #  - echo "os Go"

  # execPhp:
  #  - echo "is php"

  # 开启监听后自动执行一次上面 exec 配置的全部命令
  startup: true

# 本地静态服务器
http:
  # 类型: vue-run, vue-spa, web, 留空表示不启动
  type: none
  # 指定端口，0 表示随机可用端口
  port: 0
  # web 服务器目录
  root: ./
  # 将非本地文件的请求代理到服务器，主要用于本地跨域问题，留空表示不使用，格式如下：
  # proxy: https://blog.73zls.com
  proxy:
  # 自动打开浏览器
  openBrowser: true

# 其他
other:
  # 延迟执行指令通知时间（毫秒），不限制为 0
  delayMillSecond: 100
`

func GetExampleConfig(version string) string {
	return fmt.Sprintf(ExampleConfig, version, time.Now().Unix())
}

func GetExampleWatchConfig(version string) string {
	name := "./tmpApp"
	if zutil.IsWin() {
		name += ".exe"
	}
	return fmt.Sprintf(ExampleWatchConfig, version, name, name)
}

func GetExampleStressConfig(version string) string {
	return fmt.Sprintf(ExampleStressConfig, version)
}
