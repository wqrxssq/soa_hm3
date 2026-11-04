#!/bin/sh
set -eu

cd "$(dirname "$0")/.."

PROTOC_VERSION=29.3
TOOLS="$(pwd)/.tools"
mkdir -p "$TOOLS"

os=$(uname -s)
arch=$(uname -m)
case "$os-$arch" in
  Darwin-arm64) archive="protoc-${PROTOC_VERSION}-osx-aarch_64.zip" ;;
  Darwin-x86_64) archive="protoc-${PROTOC_VERSION}-osx-x86_64.zip" ;;
  Linux-x86_64) archive="protoc-${PROTOC_VERSION}-linux-x86_64.zip" ;;
  Linux-aarch64 | Linux-arm64) archive="protoc-${PROTOC_VERSION}-linux-aarch_64.zip" ;;
  *)
    echo "unsupported platform $os-$arch" >&2
    exit 1
    ;;
esac

if [ ! -x "$TOOLS/bin/protoc" ]; then
  curl -fsSL -o "$TOOLS/protoc.zip" \
    "https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/${archive}"
  unzip -qo "$TOOLS/protoc.zip" -d "$TOOLS"
  rm -f "$TOOLS/protoc.zip"
fi

GOBIN="$TOOLS/bin" go install google.golang.org/protobuf/cmd/protoc-gen-go google.golang.org/grpc/cmd/protoc-gen-go-grpc
export PATH="$TOOLS/bin:$PATH"
PROTO_INCLUDE="$TOOLS/include" sh scripts/protoc.sh

mkdir -p internal/booking/api
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
  --config api/openapi/codegen.yaml \
  -o internal/booking/api/openapi.gen.go \
  api/openapi/openapi.yaml

gofmt -w internal/flightv1 internal/booking/api/openapi.gen.go
