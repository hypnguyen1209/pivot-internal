#!/bin/bash

GOOS=windows GOARCH=amd64 go build -o ./build/pivot-internal.exe
GOOS=windows GOARCH=386 go build -o ./build/pivot-internal_386.exe
GOOS=darwin GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-s -w -extldflags=-static" -o ./build/pivot-internal_amd64_darwin
GOOS=darwin GOARCH=arm64 go build -a -installsuffix cgo -ldflags="-s -w -extldflags=-static" -o ./build/pivot-internal_arm64_darwin
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-s -w -extldflags=-static" -o ./build/pivot-internal_amd64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo -ldflags="-s -w -extldflags=-static" -o ./build/pivot-internal_arm64