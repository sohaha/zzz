#!/bin/bash
echo "Please select the compiled platform version"
# export CGO_ENABLED=0

# shellcheck disable=SC2006
BuildTime=`date +'%Y.%m.%d %H:%M:%S'`
BuildGoVersion="$(go version)"
LdFlags="-X 'github.com/sohaha/zzz/cmd.buildTime=${BuildTime}' -X 'github.com/sohaha/zzz/cmd.buildGoVersion=${BuildGoVersion}' "
NAME="zzz"
GOOS_D="Darwin/amd64"
GOOS_L="Linux/amd64"
GOOS_W="Windows/amd64"
GOOS_A=$(uname)/$(uname -m)
GOOS_Z="All"
GOARCH=amd64
__CGO_EN__=0
select opt in $GOOS_A" (auto)" $GOOS_D $GOOS_L $GOOS_W $GOOS_Z;do
if [ "$opt" = $GOOS_D ];then
  GOOS=darwin
  BUILD_NAME=$NAME.sh
  break
elif [ "$opt" = $GOOS_L ];then
  GOOS=linux
  BUILD_NAME=$NAME
  break
elif [ "$opt" = $GOOS_W ];then
  GOOS=windows
  BUILD_NAME=$NAME.exe
  break
elif [ "$opt" = $GOOS_Z ];then
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$LdFlags -s -w" -o ./dist/$NAME ./
  CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$LdFlags -s -w" -o ./dist/$NAME.sh ./
  CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$LdFlags -s -w" -o ./dist/$NAME.exe ./
  echo Compiled
  exit
else
  __CGO_EN__=
  GOARCH=
  GOOS=
  echo "Compile the current version of the platform"
  BUILD_NAME=zzz
  break
fi
done

if [ "$opt" = "" ];then
  __CGO_EN__=
  GOARCH=
  GOOS=
  echo "Compile the current version of the platform"
  BUILD_NAME=zzz
fi

CGO_ENABLED=$__CGO_EN__ GOARCH=$GOARCH GOOS=$GOOS go build -ldflags "$LdFlags -s -w" -o ./dist/$BUILD_NAME ./

echo $GOOS Compiled
