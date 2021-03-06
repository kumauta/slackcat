#!/bin/bash

set -e

bin=$(dirname $0)

export GOARCH=amd64
export GOOS=linux
export GOTOOLDIR=$(go env GOROOT)/pkg/linux_amd64

go build -o slackcat_linux
