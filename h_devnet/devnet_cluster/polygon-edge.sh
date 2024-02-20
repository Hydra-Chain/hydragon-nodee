#!/bin/sh

set -e

# Check if jq is installed. If not exit and inform user.
if ! command -v jq >/dev/null 2>&1; then
  echo "The jq utility is not installed or is not in the PATH. Please install it and run the script again."
  exit 1
fi

POLYGON_EDGE_BIN=polygon-edge
CHAIN_CUSTOM_OPTIONS=$(
  tr "\n" " " <<EOL
--block-gas-limit 10000000
--epoch-size 500
--chain-id 8844
--name polygon-edge-docker
--premine 0x211881Bb4893dd733825A2D97e48bFc38cc70a0c:0x314dc6448d932ae0a456589c0000
--premine 0x8c293C5b70b6493856CF4C7419E1Fb137b97B25d:0xD3C21BCECCEDA1000000
--proxy-contracts-admin 0x211881Bb4893dd733825A2D97e48bFc38cc70a0c
EOL
)

case "$1" in
"init")
  # Check if secrets already exist
  if [ -f /data/data-1/libp2p/libp2p.key ]; then
    echo "PolyBFT secrets already exist, skipping secret generation."

    # Loop through each data directory and delete specific subdirectories
    for i in 1 2 3 4 5; do
      echo "Cleaning up /data/data-$i directory..."
      rm -rf /data/data-$i/blockchain /data/data-$i/consensus/polybft /data/data-$i/consensus/validator.sig /data/data-$i/trie

      # This will generate new signatures without modifying the keys that are already present
      "$POLYGON_EDGE_BIN" polybft-secrets init --insecure --chain-id 8844 --num 5 --data-dir /data/data- --json
    done
  else
    echo "Generating PolyBFT secrets..."
  fi

  secrets=$("$POLYGON_EDGE_BIN" polybft-secrets init --insecure --chain-id 8844 --num 5 --data-dir /data/data- --json)

  rm -f /data/genesis.json

  echo "Generating PolyBFT genesis file..."
  "$POLYGON_EDGE_BIN" genesis $CHAIN_CUSTOM_OPTIONS \
    --dir /data/genesis.json \
    --consensus polybft \
    --validators-path /data \
    --validators-prefix data- \
    --reward-wallet 0xDEADBEEF:1000000 \
    --native-token-config "Hydra Token:HYDRA:18:true:$(echo "$secrets" | jq -r '.[0] | .address')" \
    --bootnode "/dns4/node-1/tcp/1478/p2p/$(echo "$secrets" | jq -r '.[0] | .node_id')" \
    --bootnode "/dns4/node-2/tcp/1478/p2p/$(echo "$secrets" | jq -r '.[1] | .node_id')" \
    --bootnode "/dns4/node-3/tcp/1478/p2p/$(echo "$secrets" | jq -r '.[2] | .node_id')" \
    --bootnode "/dns4/node-4/tcp/1478/p2p/$(echo "$secrets" | jq -r '.[3] | .node_id')" \
    --bootnode "/dns4/node-5/tcp/1478/p2p/$(echo "$secrets" | jq -r '.[4] | .node_id')"
  ;;
*)
  echo "Executing polygon-edge..."
  exec "$POLYGON_EDGE_BIN" "$@"
  ;;
esac
