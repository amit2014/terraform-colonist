#!/bin/bash
exec go run $GOPATH/src/github.com/uber/terraform-colonist/colonist/cli/colony/main.go "$@"
