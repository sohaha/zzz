# zzz 

日常开发辅助工具 *- Daily development aids*

## 安装

**Linux or MacOS**

一键安装

```shell
sudo curl -L https://raw.githubusercontent.com/sohaha/zzz/master/install.sh | bash  
```

手动安装

*[下载对应版本压缩包](https://github.com/sohaha/zzz/releases)*

```
# 解压
tar zxvf zzz_1.0.0_Linux_x86_64.tar.gz
# 执行权限
sudo chmod +x ./zzz
# 执行安装到系统（可不安装）
./zzz more install
# 测试执行
zzz help
```


**Windows**

需要手动下载 [zzz_Windows_x86_64.tar.gz
](https://github.com/sohaha/zzz/releases)，

然后打开 cmd 执行 `zzz.exe more install` 或者 设置自行环境变量。

## 使用

```shell
# 查看所有命令
zzz help
```


## 其他

如果下载安装很慢，可以尝试修改下 host 来提速，如：

```bash
sudo echo "52.217.32.124 github-production-release-asset-2e65be.s3.amazonaws.com" >> /etc/hosts
```
