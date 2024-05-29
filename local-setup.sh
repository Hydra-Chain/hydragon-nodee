#!/bin/bash

# # change directory to core-contracts
# cd ./core-contracts

# # execute hardhat compile
# npx hardhat compile

# # change directory back to parent directory
# cd ./..

# delete specified directories in test-chain-1, test-chain-2, test-chain-3, and test-chain-4
for i in 1 2 3 4 5; do
  rm -rf ./test-chain-$i/blockchain
  rm -rf ./test-chain-$i/consensus/hydragon
  rm -rf ./test-chain-$i/trie
done
