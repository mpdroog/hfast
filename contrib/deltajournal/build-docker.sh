#!/bin/bash
# https://github.com/docker-library/docs/tree/master/golang
# OLD docker run --rm -v "$PWD":/usr/src/deltajournal -w /usr/src/deltajournal golang:1.11 bash -c "
set -euo pipefail
IFS=$'\n\t'

docker run --rm -v "$PWD":/go/src/deltajournal -w /go/src/deltajournal golang:1.11 bash -c "
set -euo pipefail
IFS=$'\n\t'
/usr/bin/apt-get update
apt-get install -y libsystemd-dev
go get
go build"

scp deltajournal mark@192.168.178.44:~
