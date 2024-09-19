#!/bin/bash

# Stop execution on error
set -e

# Run secrets.sh
./secrets.sh

# Run secrets.sh
./genesis.sh

# Execute specified hydra command with all arguments
exec hydra "$@"
