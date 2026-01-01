#!/bin/bash

set -e

# 获取系统信息
os=$(uname -s)
arch=$(uname -m)
current=$PWD

# 创建随机临时目录（兼容 macOS/Linux）
tmp_dir=""
if tmp_dir=$(mktemp -d 2>/dev/null); then
  :
else
  tmp_dir=$(mktemp -d -t zzz.XXXXXXXXXX)
fi
trap 'rm -rf "$tmp_dir"' EXIT
cd "$tmp_dir"

isChinaProxy=""
echo "检测网络环境..."
isChina=$(curl --silent --connect-timeout 5 "cip.cc" | grep "中国" || echo "")
if [[ -z $isChina || "--no-china" == $1 || "1" == $NoChina ]]; then
  echo "使用国际网络"
  isChinaProxy=""
else
  echo "检测到中国网络环境"
  # isChinaProxy="https://ghproxy.com/"
fi

echo "获取最新版本..."
MAX_RETRY=3
retry_count=0
LAST_VERSION=""

auth_header=""
if [[ -n "$GITHUB_TOKEN" ]]; then
  echo "使用 GitHub Token 进行 API 请求"
  auth_header="-H \"Authorization: token $GITHUB_TOKEN\""
fi

while [[ -z $LAST_VERSION && $retry_count -lt $MAX_RETRY ]]; do
  if [[ -n "$GITHUB_TOKEN" ]]; then
    LAST_VERSION=$(curl --silent --connect-timeout 10 --max-time 30 -H "Authorization: token $GITHUB_TOKEN" "${isChinaProxy}https://api.github.com/repos/sohaha/zzz/releases/latest" | grep "tag_name" | cut -d '"' -f 4 | cut -d 'v' -f 2 || echo "")
  else
    LAST_VERSION=$(curl --silent --connect-timeout 10 --max-time 30 "${isChinaProxy}https://api.github.com/repos/sohaha/zzz/releases/latest" | grep "tag_name" | cut -d '"' -f 4 | cut -d 'v' -f 2 || echo "")
  fi
  
  if [[ -z $LAST_VERSION ]]; then
    retry_count=$((retry_count + 1))
    echo "获取版本失败，正在重试 ($retry_count/$MAX_RETRY)..."
    sleep 2
  fi
done

if [[ -z $LAST_VERSION ]]; then
  echo "获取版本失败，请检查网络连接"
  exit 1
fi

echo "最新版本: v${LAST_VERSION}"

if [[ "aarch64" == $arch ]]; then
  arch="arm64"
elif [[ "x86_64" == $arch ]]; then
  arch="amd64"
elif [[ "i386" == $arch ]]; then
  arch="386"
elif [[ "armv7l" == $arch ]]; then
  arch="arm"
fi

case "$os" in
  Darwin) os="darwin" ;;
  Linux)  os="linux" ;;
  *) echo "不支持的系统: $os"; exit 1 ;;
esac

F="zzz_${LAST_VERSION/v/}_${os}_${arch}.tar.gz"
download_url="${isChinaProxy}https://github.com/sohaha/zzz/releases/download/v${LAST_VERSION}/$F"

echo "下载文件: $F ..."
retry_count=0
download_success=false

while [[ $download_success == false && $retry_count -lt $MAX_RETRY ]]; do
  if curl -O -L --connect-timeout 10 --max-time 300 "$download_url"; then
    download_success=true
  else
    retry_count=$((retry_count + 1))
    if [[ $retry_count -lt $MAX_RETRY ]]; then
      echo "下载失败，正在重试 ($retry_count/$MAX_RETRY)..."
      sleep 2
    fi
  fi
done

if [[ $download_success == false ]]; then
  echo "下载失败，请检查网络连接或手动下载: $download_url"
  exit 1
fi

function cpzzz() {
  local target_dir="$1"
  
  if [[ ! -d "$target_dir" ]]; then
    echo "目标目录不存在: $target_dir"
    return 1
  fi
  
  if command -v sudo &> /dev/null && [[ ! -w "$target_dir" ]]; then
    echo "使用 sudo 复制文件到 $target_dir"
    sudo cp -f zzz "$target_dir/zzz"
    sudo chmod +x "$target_dir/zzz"
  else
    echo "复制文件到 $target_dir"
    cp -f zzz "$target_dir/zzz"
    chmod +x "$target_dir/zzz"
  fi
  
  return $?
}


echo "解压文件..."
if ! tar zxf "$F"; then
  echo "解压失败"
  exit 1
fi


echo "选择安装目录..."
CANDIDATES=("/usr/local/bin")
if [[ "$os" == "darwin" ]]; then
  CANDIDATES=("/opt/homebrew/bin" "/usr/local/bin")
fi
IFS=':' read -ra PATH_DIRS <<< "$PATH"
for dir in "${PATH_DIRS[@]}"; do
  CANDIDATES+=("$dir")
done

success=0
for dir in "${CANDIDATES[@]}"; do
  if [[ -z "$dir" || ! -d "$dir" ]]; then
    continue
  fi
  if cpzzz "$dir"; then
    success=1
    P="$dir"
    echo "成功安装到 $dir"
    break
  fi
done

if [[ $success -eq 0 ]]; then
  echo "无法安装到常见目录，请检查权限或尝试使用 sudo"
  echo "已将二进制复制到当前目录: $current"
  P="$current"
  cpzzz "$P" || true
fi


echo "验证安装..."
if command -v "${P}/zzz" &> /dev/null; then
  echo "安装完成！"
  "${P}/zzz" --help
else
  echo "安装可能失败，请手动验证"
fi

exit 0