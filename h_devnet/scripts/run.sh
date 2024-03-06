#!/bin/bash

# Stop execution on error
set -e

# Run secrets.sh
./secrets.sh

# Run secrets.sh
./genesis.sh

# Execute specified hydra command with all arguments
exec hydra server --data-dir ./node --chain genesis.json --grpc-address 127.0.0.1:9632 --libp2p 0.0.0.0:1478 --jsonrpc 0.0.0.0:8545 --seal --prometheus 0.0.0.0:5001 --log-level DEBUG --json-rpc-block-range-limit 0
