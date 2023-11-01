#!/bin/sh

echo PWD: $PWD
go env GOOS GOARCH GOROOT GOPATH

# GOTRACKBACK=all

go build ./latlearn/latlearn.go && go build ./example-app1.go && go build ./example-app2.go && go build ./example-app3.go && go build ./example-app4.go && go build example-app5.go && ./example-app1 && ./example-app2 && ./example-app3 && ./example-app4 && ./example-app5

#go build ./latlearn/latlearn.go && go build ./example-app4.go && ./example-app4

#go build ./latlearn/latlearn.go && go build ./example-app5.go && ./example-app5

