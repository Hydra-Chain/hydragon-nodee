#!/bin/bash

# Genesis script
main() {
  GENESIS_URL="https://raw.githubusercontent.com/Hydra-Chain/hydragon-assets/main/genesis.json"

  # Fetch and save genesis.json in the current directory
  curl -o genesis.json $GENESIS_URL
}

main
