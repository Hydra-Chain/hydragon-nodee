#!/bin/bash

# Define the directory structure
NODE_DIR="node"
CONS_DIR="${NODE_DIR}/consensus"
LIBP2P_DIR="${NODE_DIR}/libp2p"
FLAG_FILE="${NODE_DIR}/.secrets_setup_done"

# Function to write a secret to a file
write_secret() {
  secret=$(echo "$2" | tr -d '\n')
  echo -n "${secret}" >"$1"
}

# Check if the secrets setup has already been done
if [ -f "${FLAG_FILE}" ]; then
  echo "Secrets setup has already been done. Skipping..."
else
  # Check if the KEY environment variable is not set
  if [ -z "${KEY}" ]; then
    hydra secrets init --chain-id 8844 --data-dir node --insecure
  else
    # Ensure that all four environment variables are set
    if [ -z "${KEY}" ] || [ -z "${BLS_KEY}" ] || [ -z "${SIG}" ] || [ -z "${P2P_KEY}" ]; then
      echo "ERROR: All four environment variables (KEY, BLS_KEY, SIG, P2P_KEY) must be set."
      exit 1
    fi

    # Check if the directories exist
    if [ ! -d "${CONS_DIR}" ] || [ ! -d "${LIBP2P_DIR}" ]; then
      # Create the required directories
      mkdir -p "${CONS_DIR}"
      mkdir -p "${LIBP2P_DIR}"
    fi

    # Write the secrets to their respective files
    write_secret "${CONS_DIR}/validator.key" "${KEY}"
    write_secret "${CONS_DIR}/validator-bls.key" "${BLS_KEY}"
    write_secret "${CONS_DIR}/validator.sig" "${SIG}"
    write_secret "${LIBP2P_DIR}/libp2p.key" "${P2P_KEY}"
  fi

  # Create the flag file as a marker telling us that the secrets setup has been done
  touch "${FLAG_FILE}"
fi
