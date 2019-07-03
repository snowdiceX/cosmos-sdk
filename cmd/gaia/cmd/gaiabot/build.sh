#!/bin/sh

buildDate=`date +"%F %T %z"`
goVersion=`go version`
goVersion=${goVersion#"go version "}
blockchain="cosmos-sdk v0.34.7"

go build --ldflags "-X main.Version=v0.0.1 \
    -X main.GitCommit=$(git rev-parse HEAD) \
    -X 'main.BuidDate=$buildDate' \
    -X 'main.GoVersion=$goVersion' \
    -X 'main.Blockchain=$blockchain'" \
    -o ./gaiabot

