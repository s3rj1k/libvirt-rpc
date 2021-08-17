#!/bin/sh

# env
export GOPATH=${HOME}/.go
export PATH=$PATH:$GOPATH/bin

# requirements
go get -u github.com/libvirt/libvirt-go
go get -u github.com/libvirt/libvirt-go-xml
go get -u github.com/semrush/zenrpc
go get -u github.com/semrush/zenrpc/zenrpc
go get -u golang.org/x/time/rate
go get -u github.com/pierrec/lz4

# linters [https://github.com/alecthomas/gometalinter]
go get -u github.com/client9/misspell/cmd/misspell
go get -u github.com/GoASTScanner/gas/cmd/gas/...
go get -u github.com/golang/lint/golint
go get -u github.com/jgautheron/goconst/cmd/goconst
go get -u github.com/kisielk/errcheck
go get -u github.com/mdempsky/unconvert
go get -u github.com/opennota/check/cmd/aligncheck
go get -u github.com/opennota/check/cmd/structcheck
go get -u github.com/opennota/check/cmd/varcheck
go get -u honnef.co/go/tools/cmd/gosimple
go get -u honnef.co/go/tools/cmd/staticcheck
go get -u honnef.co/go/tools/cmd/unused
go get -u mvdan.cc/interfacer
go get -u honnef.co/go/tools/cmd/megacheck
