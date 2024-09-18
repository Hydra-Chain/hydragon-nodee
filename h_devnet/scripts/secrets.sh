#!/bin/bash

# Define the directory structure
NODE_DIR="/app/node"
CONS_DIR="${NODE_DIR}/consensus"
LIBP2P_DIR="${NODE_DIR}/libp2p"
SECRETS_CONFIG="${NODE_DIR}/secretsManagerConfig.json"

# Function to write a secret to a file
write_secret() {
  secret=$(echo "$2" | tr -d '\n')
  echo -n "${secret}" >"$1"
}

# Check if the secretsManagerConfig.json file exists
if [ ! -f "${SECRETS_CONFIG}" ]; then
  # If secretsManagerConfig.json does not exist, check if the CoinGecko API key is set
  if [ -z "${CG_KEY}" ]; then
    echo "ERROR: The CoinGecko API key (CG_KEY) must be set."
    exit 1
  else
    # Generate the secretsManagerConfig.json using the CoinGecko API key
    echo "Generating secretsManagerConfig.json using CoinGecko API key..."
    hydra secrets generate --type local --name node --extra "coingecko-api-key=${CG_KEY}"
    echo "Generated secretsManagerConfig.json."
  fi
else
  echo "secretsManagerConfig.json already exists."
fi

# Create the required directories if they don't exist
mkdir -p "${CONS_DIR}" "${LIBP2P_DIR}"

# Check if the validator key file is missing and corresponding env variable is set
if [ ! -f "${CONS_DIR}/validator.key" ]; then
  if [ -z "${KEY}" ]; then
    echo "ERROR: The KEY environment variable is not set."
    exit 1
  else
    write_secret "${CONS_DIR}/validator.key" "${KEY}"
    echo "Generated validator.key from KEY environment variable."
  fi
else
  echo "validator.key already exists."
fi

# Check if the validator BLS key file is missing and corresponding env variable is set
if [ ! -f "${CONS_DIR}/validator-bls.key" ]; then
  if [ -z "${BLS_KEY}" ]; then
    echo "ERROR: The BLS_KEY environment variable is not set."
    exit 1
  else
    write_secret "${CONS_DIR}/validator-bls.key" "${BLS_KEY}"
    echo "Generated validator-bls.key from BLS_KEY environment variable."
  fi
else
  echo "validator-bls.key already exists."
fi

# Check if the libp2p key file is missing and corresponding env variable is set
if [ ! -f "${LIBP2P_DIR}/libp2p.key" ]; then
  if [ -z "${P2P_KEY}" ]; then
    echo "ERROR: The P2P_KEY environment variable is not set."
    exit 1
  else
    write_secret "${LIBP2P_DIR}/libp2p.key" "${P2P_KEY}"
    echo "Generated libp2p.key from P2P_KEY environment variable."
  fi
else
  echo "libp2p.key already exists."
fi

# Check if the validator signature file is missing and corresponding env variable is set
if [ ! -f "${CONS_DIR}/validator.sig" ]; then
  if [ -z "${SIG}" ]; then
    echo "ERROR: The SIG environment variable is not set."
    exit 1
  else
    write_secret "${CONS_DIR}/validator.sig" "${SIG}"
    echo "Generated validator.sig from SIG environment variable."
  fi
else
  echo "validator.sig already exists."
fi
