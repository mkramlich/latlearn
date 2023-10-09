#!/bin/sh

go build ./example-app1.go ./latlearn.go && ./example-app1

go build ./example-app2.go ./latlearn.go && ./example-app2
