#!/bin/bash

echo ">>> Installing Operator SDK"
echo ">>> >>> Downloading source code"
go get -d github.com/operator-framework/operator-sdk

cd $GOPATH/src/github.com/operator-framework/operator-sdk

echo ">>> >>> Checking out version 0.10.0"
git checkout v0.10.0

echo ">>> >>> Running make tidy"
make tidy

echo ">>> >>> Running make install"
make install