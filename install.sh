#!/bin/bash

# set -e

os=$(uname -s)
arch=$(uname -m)

if [ -e /tmp/zzz ]; then
  rm -rf /tmp/zzz
fi

mkdir /tmp/zzz
cd /tmp/zzz

isChinaProxy="https://github.73zls.com/"
isChina=$(curl --silent "cip.cc" | grep "中国")
if [ "" == isChina ]; then
  isChinaProxy=""
fi

echo "Get Version..."
LAST_VERSION=$(curl --silent "${isChinaProxy}https://api.github.com/repos/sohaha/zzz/releases/latest" | grep  "tag_name" | cut -d '"' -f 4  | cut -d 'v' -f 2)
F="zzz_${LAST_VERSION/v/}_${os}_${arch}.tar.gz"

echo "Download tar.gz ..."
wget "${isChinaProxy}https://github.com/sohaha/zzz/releases/download/v${LAST_VERSION}/$F"

if [ $? -eq 0 ];then
  echo "Untar..."
  tar zxvf "$F"
  cp -f zzz /usr/local/bin
  chmod +x /usr/local/bin/zzz
  echo "Install done"
  /usr/local/bin/zzz --help
  rm -rf /tmp/zzz
else
  echo "Install fail..."
  exit
fi
