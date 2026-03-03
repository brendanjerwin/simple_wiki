// Package main is the wiki-cli command-line tool.
package main

//go:generate bash -c "mkdir -p ../../static/cli && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ../../static/cli/wiki-cli-linux-amd64 . && GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o ../../static/cli/wiki-cli-darwin-arm64 . && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o ../../static/cli/wiki-cli-linux-arm64 ."
