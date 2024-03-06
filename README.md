# Hydra Chain

## Becoming a validator

### Download Node code distribution

To begin your journey as a validator, you'll first need to obtain the software for your node. We currently support only Linux environments, and you can choose between two options:

- #### Executable

Download the executable for the Hydragon Node directly from [Github Releases](https://github.com/Hydra-Chain/hydragon-node/releases/latest).
After downloading, unzip the file. The extracted folder, named identically to the zip file, contains the `hydra` executable. To enhance convenience, you may want to move this executable to your system's bin directory to run it from anywhere.

- #### Build from source

##### Prerequisites

1. Golang 1.20 installed

##### Build steps

1. Clone the node source code from our [Github Repository](https://github.com/Hydra-Chain/hydragon-node/tree/prod) or download it from from our [latest release](https://github.com/Hydra-Chain/hydragon-node/releases/latest).

**Note!** Please make sure to check out the `prod` branch if you have opted to clone the repository.

```
git checkout prod
```

3. Open a terminal in the unarchived folder.

4. Build the node

```
CGO_ENABLED=0 go build -o hydra -a -installsuffix cgo  main.go
```

**CGO_ENABLED=0**: This environment variable disables CGO, which is a feature in Go that allows for the creation of Go packages that call C code. Setting CGO_ENABLED=0 makes the build static, meaning it does not depend on C libraries at runtime, enhancing portability across different environments without needing those C libraries installed.

**go build**: This is the Go command to compile packages and dependencies into a binary executable.

**-o hydra**: The -o flag specifies the output file name for the compiled binary. In this case, the binary will be named `hydra`.

**-a**: This flag forces a rebuild of all packages that are part of the binary. It's useful to ensure you're compiling with the most up-to-date version of the code and dependencies.

**-installsuffix cgo**: This flag adds a suffix to the package path of the compiled binary. This is often used in conjunction with disabling CGO to differentiate the compiled packages from those built with CGO enabled. It can help avoid conflicts and ensure the correct packages are used.

**main.go**: This specifies the main entry file of the application to be compiled. It contains the main function, which is the starting point of the Go program.

To build Go applications for different platforms directly from your command line, set `GOOS` for the target operating system (e.g., darwin, linux, windows) and `GOARCH` for the architecture (e.g., amd64, arm64). Without specifying these variables, the Go compiler defaults to the current machine's OS and architecture.

Example:

```
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o hydra -a -installsuffix cgo  main.go
```

4. Add the generated binary to your system's PATH environment variable to allow its execution from any directory.

- #### Docker image

Alternatively, pull the Docker image for Hydragon from our repository at [Docker Hub](https://hub.docker.com/repository/docker/rsantev/hydra-client/general). This method is ideal for users who prefer containerized environments.

### Generate secrets

The foundation of your validator identity is built upon three distinct private keys:

- ECDSA Key: Your main interaction tool with the blockchain, allowing you to perform transactions.
- BLS Key: Enables your participation in the consensus mechanism as a validator, authorizing you to sign various block data.
- ECDSA Networking Key: Facilitates peer-to-peer communication with other nodes within the network.

There are different options on how and where the secrets to be stored but we recommend storing the keys encrypted on your local file system and maintaining offline backups. To generate these keys, use the following command, which encrypts them locally:

```
hydra secrets init --chain-id 8844 --data-dir node-secrets
```

This command initiates the creation of your node's secrets for a testnet chain with ID 8844, storing them in a directory named node-secrets. During this process, you'll confirm each secret and establish a password for file encryption.

Successful execution results in a summary of your generated and initialized secrets, including your public key (address), BLS public key, and node ID.

```
[SECRETS GENERATED]
network-key, validator-key, validator-bls-key, validator-bls-signature

[SECRETS INIT]
Public key (address) = 0x58835Fe9f000B49fAd7146E0Eb4DF295715EaE63
BLS Public key       = 048649f153a668b86043b1ab6e2d33a91cff389df100666c5aeb5c4ab8c6e8e20fada19085edf4a6dd0814251b4c9ac42c158d407e754066cf6e77d18a20a1ab0c8bd814a06489eafdbc03f17b12dd786001313493b4f880bc5c0a1cf6bcac871d28a030c6f1b3673b686f67b5e24ca8cf88cd6838e9c77ab44f2a1bdc33bde9
Node ID              = 16Uiu2HAmQ1kj6B9PM1K5DorkwHhgWLzKSiL6VEZLxharhCbNTXKU
```

### Configuring your node

#### The Genesis File

The genesis.json file is crucial, containing details about the genesis block and node configurations.
**Important: Do not alter this file to avoid potential loss of funds.**
Future releases will automate this configuration. You can find the Testnet genesis file in the extracted folder containing the [release assets](#executable) and place it in your node directory.

#### Secrets Configuration File

The next step is to configure the secrets-config.json file that tells the node that encrypted local secrets are used.

You can find the secrets-config.json file in the extracted folder containing the [release assets](#executable).
Paste the file in your node directory.

### Launching the Node

Run your node with the following command from its directory:

```
hydra server --data-dir ./node-secrets --chain ./genesis.json --grpc-address :9632 --libp2p 0.0.0.0:1478 --jsonrpc 0.0.0.0:8545  --secrets-config ./secrets-config.json
```

This process may take some time, as the node needs to fully sync with the blockchain. Once the syncing process is complete, you will need to restart the node by running the same command.

### Prepare account to be a validator

Once your node is operational and fully synced, you're ready to become a validator. This requires:

- Funding Your Account: Obtain sufficient Hydra by visiting the [Faucet](#faucet) section.

**Note:** Currently, you will have to import the validator's private key into Metamask to be able to interact with the web UI which can be considered as a security issue, but we will provide better option in the future.

- Whitelisting: Your public key needs to be whitelisted by the Hydra team to participate as a validator. Use the command below to retrieve your public key, then forward it to the Hydra team for whitelisting:

Check your public secrets data with the following command:

```
hydra secrets output --data-dir node-secrets
```

You need the following value:

```
[SECRETS INIT]
Public key (address) = 0x...
```

Send it to Hydra's team, so they can whitelist your address to be able to participate as validator.

### Register account as validator and stake

After Hydra's team confirms you are whitelisted you have to register your account as a validator and stake a given amount.

```
hydra polybft register-validator --data-dir ./node-secrets --stake 99000000000000000000 --chain-id 8844 --jsonrpc http://localhost:8545
```

The above command both register the validator and stakes the specified amount.

Use the following command in case you want to execute the stake operation only:

```
hydra polybft stake --data-dir ./node-secrets --self true --amount 99000000000000000000 --jsonrpc http://localhost:8545
```

**Note:** Amounts are specified in wei.

Congratulations! You have successfully become a validator on the Hydra Chain. For further information and support, join our Telegram group and engage with the community.

## Becoming a delegator

We've implemented the initial version of a straightforward dashboard, enabling users to connect their wallet, request testing HYDRA coins from our Faucet, and access to [delegation](#delegation) section where one can delegate funds to validators. To access the Dashboard Interface, please visit [https://app.testnet.hydrachain.org](https://app.testnet.hydrachain.org).

### Adding Hydragon network to Metamask

In this section, we will explain how to add the Hydragon network to your Metamask wallet extension:

- Navigate to your Metamask extension and click on the **network selector button**. This will display a list of networks that you've added already.
- Click `Add network` button
- MetaMask will open in a new tab in fullscreen mode. From here, find and the `Add network manually` button at the bottom of the network list.
- Complete the fields with the following information:

**Network name:**

```
Hydra Testnet (or any name that suits you)
```

**New RPC URL:**

```
https://rpc.testnet.hydrachain.org
```

**Chain ID:**

```
8844
```

**Currency symbol:**

```
tHYDRA
```

**Block explorer URL (Optional):**

```
https://hydragon.hydrachain.org
```

- Then, click `Save` to add the network.
- After performing the above steps, you will be able to see the custom network when you access the network selector (same as the first step).

### Faucet

In the Faucet section, users have the option to request a fixed amount of test HYDRA coins, granting them opportunity to explore the staking/delegation processes and other features. Please note that there will be a waiting period before users can request test tokens again.

- Navigate to [https://app.testnet.hydrachain.org/faucet](https://app.testnet.hydrachain.org/faucet) to access the Faucet section of our platform.
- Here, users can connect their wallet and request HYDRA coins.
- To connect a self-custody wallet (e.g., Metamask), click the `Connect` button located in the top right corner or within the Faucet form.
- Once the wallet is connected, Click on `Request HYDRA` to receive 100 HYDRA to the connected wallet. Please be aware that there is a 2-hour cooldown before additional coins can be requested.

### Delegation

In the Delegation section, users can interact with an intuitive UI to delegate to active validators. There are two types of delegation available: normal delegation, which can be undelegated at any time. It offers a fixed APR. There is also a vested position delegation, which includes a lockup mechanism. With vested delegation, users can potentially earn up to almost 80% APR, depending on different economical parameters. It's important to note that penalties apply for undelegating from a still active vested positions. More details regarding APR calculations and vested delegation can be found in our upcoming public paper. Here's how to proceed:

- Navigate to [https://app.testnet.hydrachain.org/delegation](https://app.testnet.hydrachain.org/delegation) to access the Delegation section of our platform. If you're already on the platform, you can find the `Delegation` section in the sidebar on the left.

- Upon entering the Delegation section, you'll find an overview of your delegation, including the number of validators, the current APR, the total delegated HYDRA, and a table listing all the validators. Click on Actions (Details) in order to see more details for the selected validator.

- Clicking on details will open a dashboard displaying the validator's total delegated amount, voting power, and your own delegation, if any. The table below shows all open positions for this validator. On the right side of the table, you'll find buttons to Delegate, Undelegate, Claim, or Withdraw from a selected position.

- To delegate, click the `Delegate` button. A new window will appear showing the available HYDRA in your wallet, the amount currently delegated to the selected position (if any), whether the position is vested, and the option to choose the lockup period in weeks.

**Note:** When creating a vested position, the web UI will prompt you to execute 2 separate transactions.

You'll also see the potential APR calculated based on the vesting period, or you can opt for the default APR. Enter the amount you wish to delegate, review the amount of LYDRA you'll receive, and the approximate network fee. Select the amount and click `Delegate`.

- After clicking `Delegate`, a new window will display the transaction status, and the platform will prompt you to connect your wallet for signature. Confirm the transaction in your wallet, and once confirmed, the status will change to `Transaction confirmed`. You can then close the windows to view the new position in the table below. It could take a while until the transaction is confirmed, but you can safely leave the page and once the process is completed, and you come back, the new delegate position will appear in the table.

- To undelegate, simply click the `Undelegate` button. A modal will appear where you can see the available LYDRA, the amount delegated in this position, and a field to enter the amount you wish to undelegate. You can also use the `MAX` button to fill in the full delegated amount automatically. Note that you must have the same amount of Lydra available in your wallet to proceed the undelegation.

- Below, you'll see the amount of HYDRA you'll receive and the approximate network fee for the transaction. If the position is vested and still active, a warning message will appear, indicating that the vesting period isn't over yet. It will also provide an approximate calculation of the penalty for undelegating early, and reminder that all rewards will be forfeited.

- The process for pending transactions and confirmations remains the same. Once the transaction is confirmed, the table will be updated to reflect the remaining staked amount, if any."
