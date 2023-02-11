#!/bin/bash

for filename in ./api/clients/*.proto; do
    protoc --go_out=internal/generated/rpc/clients --go-grpc_out=internal/generated/rpc/clients $filename
done
