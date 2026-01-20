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
   tar zxvf zzz_1.0.0_Linux_x86_64.tar.gz
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

手动下载 [zzz_Windows_x86_64.tar.gz](https://github.com/sohaha/zzz/releases)（[国内镜像](https://github.73zls.com/sohaha/zzz/releases)）。

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

## 开发工具链

推荐使用 mise 统一工具链版本：

```bash
mise install
```

## 使用

```shell
# 查看所有命令
zzz help
```

## 致谢

- [YXVM](https://yxvm.com/aff.php?aff=765) 赞助
- [NodeSupport](https://github.com/NodeSeekDev/NodeSupport) 赞助

<div align="center">
  <p>⭐ 如果你喜欢这个项目，请给它一个星标！⭐</p>
</div>
