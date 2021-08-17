#!/bin/sh

# env
export GOPATH=${HOME}/.go
export PATH=$PATH:$GOPATH/bin

BuildDate=`TZ=UTC date -u '+%Y-%m-%dT%H:%M:%SZ'`

GitBranch=`git symbolic-ref --short HEAD`

GitState=`git status --porcelain \
            | awk 'BEGIN{i=0}{i++;} END{ if(i>0){print "dirty"}else{print "clean"} }'`

GitCommit=`git rev-parse --short HEAD`

# count lines of code
echo -n "Lines of Code: "
ls -1 | grep -E '.go$' | grep -v 'main_zenrpc.go' | xargs cat | sed '/^\s*$/d' | wc -l

go generate && go build -a -ldflags \
    "-s -X main.buildDate=${BuildDate} -X main.gitBranch=${GitBranch} -X main.gitState=${GitState} -X main.gitCommit=${GitCommit}"
