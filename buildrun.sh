#!/bin/sh

go build ./example-app1.go ./latlearn.go && ./example-app1 && cat ./latency-report*.txt
