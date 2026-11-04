#!/bin/sh
set -eu

cd "$(dirname "$0")/.."

protoc \
  -I api/proto \
  -I "${PROTO_INCLUDE:?PROTO_INCLUDE is required}" \
  --go_out=. --go_opt=module=flightbooking \
  --go-grpc_out=. --go-grpc_opt=module=flightbooking \
  api/proto/flight/v1/flight.proto
