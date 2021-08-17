#!/bin/sh

# env
export GOPATH=${HOME}/.go
export PATH=$PATH:$GOPATH/bin

go generate
aligncheck
errcheck ./
go vet
goconst -min-occurrences=3 -min-length=5 ./
golint
gosimple
interfacer
misspell ./
staticcheck
structcheck
unconvert -v
unused
varcheck
go fmt

gas -quiet ./
# megacheck

# git diff --word-diff=color
