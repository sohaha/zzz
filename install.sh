#!/bin/bash

set -e

os=$(uname -s)
arch=$(uname -m)

if [ -e /tmp/zzz ]; then
  rm -rf /tmp/zzz
fi

mkdir /tmp/zzz
cd /tmp/zzz

echo "Get Version..."
LAST_VERSION=$(curl --silent "https://api.github.com/repos/sohaha/zzz/releases/latest" | grep -Po '"tag_name": "\K.*?(?=")')
F="zzz_${LAST_VERSION/v/}_${os}_${arch}.tar.gz"

echo "Download tar.gz ..."
wget "https://github.com/sohaha/zzz/releases/download/${LAST_VERSION}/$F"

echo "Untar..."
tar zxvf "$F"


cp -f zzz /usr/bin/
chmod +x /usr/bin/zzz

echo "Install done"

/usr/bin/zzz --help

rm -rf /tmp/zzz
