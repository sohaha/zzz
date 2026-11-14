# zzz

日常开发辅助工具

## 安装

**Linux / macOS**

一键安装：

```bash
curl -L https://raw.githubusercontent.com/sohaha/zzz/master/install.sh | bash
```

手动安装：

1. [下载对应版本压缩包](https://github.com/sohaha/zzz/releases)
2. 解压：
   ```bash
   tar zxvf zzz_<版本>_linux_amd64.tar.gz
   ```
3. 赋予可执行权限：
   ```bash
   sudo chmod +x ./zzz
   ```
4. 安装到系统（可选）：
   ```bash
   ./zzz more install
   ```
5. 测试：
   ```bash
   zzz help
   ```

**Windows**

手动下载 `zzz_<版本>_windows_amd64.tar.gz`（[Releases](https://github.com/sohaha/zzz/releases)，[国内镜像](https://github.73zls.com/sohaha/zzz/releases)）。

下载后在 cmd 执行 `zzz.exe more install`，或自行配置环境变量。

**Go 安装**

本地有 Go 环境可直接：

```bash
go install github.com/sohaha/zzz@latest
```

**NPM 安装**

```bash
npm i -g @sohaha/zzz
```

## 使用

```shell
# 查看所有命令
zzz help

# 查看某个命令的参数
zzz <command> --help

# 生成 shell 补全脚本
zzz completion bash
```

主配置文件默认位于 `~/.zzz/config.yaml`，首次运行时会自动创建。

## 本地开发

项目要求 Go 1.25 或更高版本。提交前建议按项目现有工具链执行以下验证：

```shell
gofmt -l .
go vet ./...
go test -race ./...
go build ./...
```

如本地安装了 GoReleaser v2，可额外校验发布配置：

```shell
goreleaser check
```

`build.sh` 是交互式本地打包脚本；GitHub Actions 会按 `go.mod` 中的 Go 版本自动执行格式化检查、静态检查、测试、构建和发布配置校验。

## 致谢

- [YXVM](https://yxvm.com/aff.php?aff=765) 赞助
- [NodeSupport](https://github.com/NodeSeekDev/NodeSupport) 赞助

<div align="center">
  <p>⭐ 如果你喜欢这个项目，请给它一个星标！⭐</p>
</div>
