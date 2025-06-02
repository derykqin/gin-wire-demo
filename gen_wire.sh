#!/bin/bash
echo "重新生成wire_gen.go"
rm -rf ./cmd/server/wire_gen.go
wire ./cmd/server
