#!/bin/bash

set -e

bin=$(dirname $0)

export GOARCH=amd64
export GOOS=darwin
export GOTOOLDIR=$(go env GOROOT)/pkg/tool/darwin_amd64

go build -o slackcat_darwin
