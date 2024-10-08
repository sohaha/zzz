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
if [[ -z $isChina || "--no-china" == $1 || "1" == $NoChina ]]; then
  isChinaProxy=""
fi

echo "Get Version..."
LAST_VERSION=$(curl --silent "${isChinaProxy}https://api.github.com/repos/sohaha/zzz/releases/latest" | grep  "tag_name" | cut -d '"' -f 4  | cut -d 'v' -f 2)

if [[ "" == $LAST_VERSION ]]; then
  echo "Failed to get version, please check the network"
  exit 1
fi


if [[ "aarch64" == $arch ]]; then
  arch="arm64"
fi

F="zzz_${LAST_VERSION/v/}_${os}_${arch}.tar.gz"

echo "Download tar.gz ..."
curl -O -L "${isChinaProxy}https://github.com/sohaha/zzz/releases/download/v${LAST_VERSION}/$F"

function cpzzz() {
    if command -v sudo &> /dev/null; then
    sudo cp -f zzz $P
    else
    cp -f zzz $P
    fi
}

if [ $? -eq 0 ];then
  echo "Untar..."
  tar zxf "$F"

  P="/usr/local/bin"

  cpzzz
  if [ $? -eq 0 ];then
    echo "Install done"
  else
    P=$(echo $PATH | cut -d ':' -f 1)
    cpzzz
  fi

  chmod +x "${P}/zzz"
  "${P}/zzz" --help
  rm -rf /tmp/zzz
else
  echo "Install fail..."
  exit
fi
