#!/bin/bash
exec go run $GOPATH/src/github.com/uber/terraform-colonist/colonist/tvm/cli/tvm/main.go "$@"
