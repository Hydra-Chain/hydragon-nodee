# Hydra Chain

## Becoming a validator

### System Requirements

This is the minimum hardware configuration required to set up a Hydra validator node:

| Component | Minimum Requirement            | Recommended                              |
| --------- | ------------------------------ | ---------------------------------------- |
| Processor | 4-core CPU                     | 8-core CPU                               |
| Memory    | 16 GB RAM                      | 32 GB RAM                                |
| Storage   | 200 GB NVME SSD                | 1 TB NVME SSD                            |
| Network   | High-speed internet connection | Dedicated server with gigabit connection |

> Note that these minimum requirements are based on the x2iezn.2xlarge instance type used in the performance tests, which demonstrated satisfactory performance. However, for better performance and higher transaction throughput, consider using more powerful hardware configurations, such as those equivalent to x2iezn.4xlarge or x2iezn.8xlarge instance types.

##### Hardware environment tips

    While we do not favor any operating system, more secure and stable Linux server distributions (like CentOS) should be preferred over desktop operating systems (like Mac OS and Windows).

    The minimum storage requirements will change over time as the network grows. It is recommended to use more than the minimum requirements to run a robust full node.

### Download Node code distribution

To begin your journey as a validator, you'll first need to obtain the software for your node. We currently support only Linux environments, and you can choose between three options:

- #### Executable

Download the executable for the Hydragon Node directly from [Github Releases](https://github.com/Hydra-Chain/hydragon-node/releases/latest).
After downloading, unzip the file. The extracted folder, named identically to the zip file, contains the `hydra` executable. To enhance convenience, you may want to move this executable to your system's bin directory to run it from anywhere.

- #### Build from source

##### Prerequisites

1. Golang >=1.21 installed. (The current recommended version is 1.23.3)

##### Build steps

1. Clone the node source code from our [Github Repository](https://github.com/Hydra-Chain/hydragon-node/tree/prod) or download it from from our [latest release](https://github.com/Hydra-Chain/hydragon-node/releases/latest).

**Note!** Please make sure to check out the `prod` branch if you have opted to clone the repository.

```
git checkout prod
```

3. Open a terminal in the unarchived folder.

Use the Makefile to build the source.

4. Build the node

```
make build
```

**CGO_ENABLED=0**: This environment variable disables CGO, a Go feature that allows Go packages to call C code. Setting CGO_ENABLED=0 creates a **static build**, meaning the resulting binary won’t rely on C libraries at runtime, increasing portability across different environments without needing C libraries installed.

**go build**: This command compiles the Go packages and their dependencies into a binary executable.

**-o hydra**: The -o flag specifies the output file name for the compiled binary. In this case, the binary will be named `hydra`.

**main.go**: This specifies the main entry file of the application to be compiled. It contains the main function, which is the starting point of the Go program.

To build Go applications for different platforms directly from your command line, update the build command by setting `GOOS` for the target operating system (e.g., darwin, linux, windows) and `GOARCH` for the architecture (e.g., amd64, arm64). Without specifying these variables, the Go compiler defaults to the current machine's OS and architecture.

Example:

```
GOOS=darwin GOARCH=arm64
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
hydra secrets init --chain-id 4488 --data-dir node-secrets
```

This command initiates the creation of your node's secrets for the HydraChain with ID 4488, storing them in a directory named node-secrets. During this process, you'll confirm each secret and establish a password for file encryption.

Successful execution results in a summary of your generated and initialized secrets, including your public key (address), BLS public key, and node ID.

```
[SECRETS GENERATED]
network-private-key, evm-private-key, validator-bls-private-key, validator-bls-signature

[SECRETS INIT]
EVM Address              = 0x3adE0971cc813A3F4e1BDaA225ab612bE57e2eaa
Validator BLS Public key = 179d7cfe2568c1cd2ba8988f3ab355008e9a3e07c89eb61210ab6f8a69e6845a218d5a91afd0681563e55330924d3f2f03769a3e6e3aa3bbad5cda3c74e10b1c2ee80f1335d2a86f2ba28641fd9a169e399f4de888e331eb413b8a69f87bbb8913dd09c7e6a997e649de5d23cf6eadd3d57926beaa2f2ead37084aed3b3230c3
Node ID                  = 16Uiu2HAm66TZf6DnK2qjLCRLQZiHttGpsHCGtdNxLb1eMgWNM6cD
```

#### Output the secret and public data for the validator

There may be situations where you need to retrieve the secret data (such as private keys) after initializing the node. You can retrieve the private keys using the following command:

```
hydra secrets output-private --data-dir node-secrets
```

You’ll then be prompted to enter the password you set during the secrets initialization process.

The same steps apply for retrieving public keys and the Node ID. Use the following command to output this information:

```
hydra secrets output-public --data-dir node-secrets
```

For more details on available commands and their usage, you can append the `--help` flag to any of them.

### Configuring your node

#### The Genesis File

The genesis.json file is crucial, containing details about the genesis block and node configurations.
**Important: Do not alter this file to avoid potential loss of funds.**
Future releases will automate this configuration. You can find the HydraChain genesis file in the extracted folder containing the [release assets](#executable) and place it in your node directory.

#### Secrets Configuration File

The next step is to configure the secretsManagerConfig.json file that tells the node that encrypted local secrets are used.
We also need an extra flag that configures communication with the CoinGecko API used to fetch price data for the Hydra Price Oracle functionality. A free API key can be generated at CoinGecko's [API page](https://www.coingecko.com/en/api).

The configuration file can be generated by running the following command:

```
hydra secrets generate --type encrypted-local --name node --extra "coingecko-api-key=<key>"
```

### Launching the Node

Run your node with the following command from its directory:

```
hydra server --data-dir ./node-secrets --chain ./genesis.json --grpc-address :9632 --libp2p 0.0.0.0:1478 --jsonrpc 0.0.0.0:8545 --secrets-config ./secretsManagerConfig.json
```

This process may take some time, as the node needs to fully sync with the blockchain. Once the syncing process is complete, you can proceed with the next steps.

### Prepare account to be a validator

After your node is operational and fully synced, you're ready to become a validator. This requires:

- (TestNet Only) Funding Your Account: Obtain sufficient Hydra by visiting the [Faucet](#faucet) section. You can obtain funds to a second metamask account and transfer them to the validator address. Or alternatively you can import the private key of your validator into Metamask to operate with it directly.

**Note:** Currently, you will have to import the validator's private key into Metamask to be able to interact with the web UI which can be considered as a security issue, but we will provide better option in the future.

### Register account as validator and stake

Hydra's validator set is unique as it offers a permissionless opportunity on a first come/first serve. It supports up to 150 validators and uses exponentiating formula to ensure consolidation is countered for a maximum Nakamoto Coefficient. The requirements to become a validator: a) to have a minimum of 15,000 HYDRA and b) there to be vacant slots in the validator sets. Inactive validators are going to be ejected after approximately 1 hours of inactivity and permanently banned after additional 24 hours if the ban process is not terminated (See the [Ban Validator](#ban-validator) section for more details.) in order to ensure fair environment and highest level of network security.

After ensuring you have a minimum of 15,000 HYDRA in your validator wallet, you can execute the following command.

```
hydra hydragon register-validator --data-dir ./node-secrets --stake 15000000000000000000000 --commission 10 --jsonrpc http://localhost:8545
```

The command above both registers the validator, stakes the specified amount and sets the commission (in percentage). This commission will be deducted from future delegators’ rewards. Change the value next to the `--commission` flag with the desired commission and ensure it is between 0 and 100.

#### Stake

Use the following command if you want to perform a stake operation only or increase your existing stake:

```
hydra hydragon stake --data-dir ./node-secrets --self true --amount 15000000000000000000000 --jsonrpc http://localhost:8545
```

**Note:** Amounts are specified in wei.

Congratulations! You have successfully become a validator on the Hydra Chain. For further information and support, join our Telegram group and engage with the community.

#### Stake with Vesting

Hydra Chain allows you to open a vested position, where your funds are locked for a chosen period between 1 and 52 weeks (1 year). In return, you receive a loyalty bonus on the APR. The longer the vesting duration, the higher the bonus.

You can unstake your funds prematurely by paying a penalty fee, which is calculated at 0.5% per remaining week of the lockup period. Consequently, the closer the position is to maturity, the lower the penalty fee, while the further away, the higher the cost. Additionally, any rewards distributed to you in the vesting period will also be burned in the process. For more details on this mechanism, refer to the Whitepaper.

```
hydra hydragon stake --data-dir ./node-secrets --self true --amount 15000000000000000000000 --vesting-period 52 --jsonrpc http://localhost:8545
```

**Note:** The amounts are specified in wei, and the specified value will be added to your existing staked amount. If you do not wish to increase your stake, set the amount value to 0.

**Note:** You can vest your already staked balance by setting the 'amount' flag to 0.

**Note:** If you have a prevoiusly staked balance and then you execute the command above with X amount, the vested balance will be old balance + X amount, meaning that the vesting is applied on the total balance.

Congratulations! Enjoy the enhanced rewards and benefits provided by Vested Staking.

### Update the commission for the delegators

After becoming a validator, you can still update the commission rate if needed. The process begins with executing a command to set the new commission as pending. This is followed by a 15-day waiting period, after which you can apply the updated commission. This waiting period is designed to prevent validators from acting dishonestly by executing commission changes shortly before delegators claim their rewards. You can initialize the new commission using the following command:

```
hydra hydragon commission --data-dir ./node-secrets --commission 15 --jsonrpc http://localhost:8545
```

After the waiting period ends, execute the following command to apply the commission rate set in the previous step:

```
hydra hydragon commission --data-dir ./node-secrets --apply true --jsonrpc http://localhost:8545
```

### Claiming the generated rewards from validating and commission rewards from delegators

Once a validator starts validating blocks, rewards are generated at the end of each epoch based on the validator’s activity level. Validators can claim rewards at any time, provided rewards have been generated. If no rewards are available, the transaction will fail. Use the following command:

```
hydra hydragon claim-rewards --data-dir ./node-secrets --jsonrpc http://localhost:8545
```

Additionally, the rewards that are generated from the claimings of the delegators, you can claim it by executing the command below:

```
hydra hydragon commission --data-dir ./node-secrets --claim true --jsonrpc http://localhost:8545
```

### Ban Validator

To reduce the risk of stalling caused by validators experiencing temporary issues or acting maliciously, we’ve implemented an ejection and ban mechanism. Anyone who recongizes a suspicious activity, and the rules are met, can execute the ban process. Below is an outline of how the system works (specific conditions are detailed in our [genesis contracts](https://github.com/Hydra-Chain/hydragon-core-contracts)):

1. **Initial Ejection**: If your validator stops proposing or participating in consensus whether due to hardware failure, software issues, or malicious intent—the ban procedure will be initiated. The validator will be ejected, allowing time for recovery. If no action is taken, a ban may follow. The threshold to trigger this process is initially set at 18,000 blocks (~2 hours), depending on block creation speed.
2. **Ban Procedure**: After ejection, you can rejoin by resolving the issue and running the appropriate command (explained [below](#re-activate)). However, if you fail to act within the final threshold (86,400 seconds or ~24 hours), your validator will be permanently banned. This will result in a penalty (currently 1,000 HYDRA) , of which 700 HYDRA will be burned and a small reward for the reporter (currently 300 HYDRA; applied only if ban is executed by reporter different than the Governance), and the remaining funds being prepared for withdrawal.

#### Re-activate

After resolving the encountered issues, you should restart the node to sync with the blockchain. Once synced, you can reactivate by running the following command:

```
hydra hydragon terminate-ban --data-dir ./node-secrets --jsonrpc http://localhost:8545
```

Congratulations, you’re back in action!

**Note:** Please keep in mind that if malicious behavior is detected, a manual ban can be initiated by the Hydra DAO. Furthermore, if the conditions for initiating a ban and enforcing the ban are met, a user can execute the relevant functions by interacting with the contract via the explorer or programmatically.

#### Withdrawing penalized funds after ban

If your validator has been banned, you can still withdraw the remaining funds after the penalty and burned rewards have been deducted. To do so, you must first initiate the withdrawal process for the penalized funds and then wait for the withdrawal period to complete. Use the following command:

```
hydra hydragon withdraw --penalized-funds true --data-dir ./node-secrets --jsonrpc http://localhost:8545
```

After the withdrawal period has ended, you can claim the remaining funds using the command below:

```
hydra hydragon withdraw --data-dir ./node-secrets --jsonrpc http://localhost:8545
```

If your validator has been banned, you can still withdraw the remaining funds after the penalty and burned rewards have been deducted. Use the following command:

```
hydra hydragon withdraw --data-dir ./node-secrets --banned --jsonrpc http://localhost:8545
```

**_Note:_** If your machine is no longer running, you can use [our RPC](#adding-hydragon-network-to-metamask) as the value for the jsonrpc flag.

### Command Line Interface

Here are the Hydra Chain node CLI commands that currently can be used:

- Usage:

```
  hydra [command]
```

- Available Commands:

| Command    | Description                                                                                                                    |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------ |
| backup     | Create blockchain backup file by fetching blockchain data from the running node                                                |
| bridge     | Top level bridge command                                                                                                       |
| completion | Generate the autocompletion script for the specified shell                                                                     |
| genesis    | Generates the genesis configuration file with the passed in parameters                                                         |
| help       | Help about any command                                                                                                         |
| hydragon   | Executes Hydra Chain's Hydragon consensus commands, including staking, unstaking, rewards management, and validator operations |
| license    | Returns Hydra Chain license and dependency attributions                                                                        |
| monitor    | Starts logging block add / remove events on the blockchain                                                                     |
| peers      | Top level command for interacting with the network peers. Only accepts subcommands                                             |
| regenesis  | Copies trie for specific block to a separate folder                                                                            |
| secrets    | Top level SecretsManager command for interacting with secrets functionality. Only accepts subcommands                          |
| server     | The default command that starts the Hydra Chain client by bootstrapping all modules together                                   |
| status     | Returns the status of the Hydra Chain client                                                                                   |
| txpool     | Top level command for interacting with the transaction pool. Only accepts subcommands                                          |
| version    | Returns the current Hydra Chain client version                                                                                 |

- Flags:

| Flag       | Description                                    |
| ---------- | ---------------------------------------------- |
| -h, --help | help for this command                          |
| --json     | get all outputs in json format (default false) |

- Use "hydra [command] --help" for more information about a command.

## Becoming a delegator

We've implemented the initial version of a straightforward Staking dashboard, where one can delegate funds to validators. To access the Dashboard Interface, please visit [stake.hydrachain.org](https://stake.hydrachain.org/).

### Adding Hydragon network to Metamask

In this section, we will explain how to add the Hydragon network to your Metamask wallet extension:

- Navigate to your Metamask extension and click on the **network selector button**. This will display a list of networks that you've added already.
- Click `Add network` button
- MetaMask will open in a new tab in fullscreen mode. From here, find and the `Add network manually` button at the bottom of the network list.
- Complete the fields with the following information:

**Network name:**

```
Hydra Chain
```

**New RPC URL:**

```
https://rpc-mainnet.hydrachain.org
```

**Chain ID:**

```
4488
```

**Currency symbol:**

```
HYDRA
```

**Block explorer URL (Optional):**

```
https://skynet.hydrachain.org
```

- Then, click `Save` to add the network.
- After performing the above steps, you will be able to see the custom network when you access the network selector (same as the first step).

### Faucet (TestNet Only)

In the Faucet section, users have the option to request a fixed amount of test HYDRA coins, granting them opportunity to explore the staking/delegation processes and other features. Please note that there will be a waiting period before users can request test tokens again.

- Navigate to [testnetapp.hydrachain.org/faucet](https://testnetapp.hydrachain.org/faucet) to access the Faucet section of our platform.
- Here, users can connect their wallet and request HYDRA coins.
- To connect a self-custody wallet (e.g., Metamask), click the `Connect` button located in the top right corner or within the Faucet form.
- Once the wallet is connected, Click on `Request HYDRA` to receive 16000 HYDRA to the connected wallet. Please be aware that there is a 2-hour cooldown before additional coins can be requested.

### Delegation

In the Delegation section, users can interact with an intuitive UI to delegate to active validators. There are two types of delegation available: normal delegation, which can be undelegated at any time. It offers a fixed APR. There is also a vested position delegation, which includes a lockup mechanism. With vested delegation, users can potentially earn up to almost 80% APR, depending on different economical parameters. It's important to note that penalties apply for undelegating from a still active vested positions. More details regarding APR calculations and vested delegation can be found in our upcoming public paper. Here's how to proceed:

- Navigate to [stake.hydrachain.org](https://stake.hydrachain.org) to access the Delegation section of our platform. If you're already on the platform, you can find the `Delegation` section in the sidebar on the left.

- Upon entering the Delegation section, you'll find an overview of your delegation, including the number of validators, the current APR, the total delegated HYDRA, and a table listing all the validators. Click on Actions (Details) in order to see more details for the selected validator.

- Clicking on details will open a dashboard displaying the validator's total delegated amount, voting power, and your own delegation, if any. The table below shows all open positions for this validator. On the right side of the table, you'll find buttons to Delegate, Undelegate, Claim, or Withdraw from a selected position.

- To delegate, click the `Delegate` button. A new window will appear showing the available HYDRA in your wallet, the amount currently delegated to the selected position (if any), whether the position is vested, and the option to choose the lockup period in weeks.

**Note:** When creating a vested position, the web UI will prompt you to execute 2 separate transactions.

You'll also see the potential APR calculated based on the vesting period, or you can opt for the default APR. Enter the amount you wish to delegate, review the amount of LYDRA you'll receive, and the approximate network fee. Select the amount and click `Delegate`.

- After clicking `Delegate`, a new window will display the transaction status, and the platform will prompt you to connect your wallet for signature. Confirm the transaction in your wallet, and once confirmed, the status will change to `Transaction confirmed`. You can then close the windows to view the new position in the table below. As mentioned above, if the position is vested, you will be prompted for a second transaction slightly after the first one. It could take a while until the transaction is confirmed, but you can safely leave the page and once the process is completed, and you come back, the new delegate position will appear in the table.

- Under `MY DELEGATION` section, there is a `MY EARNINGS` section where one can see the rewards generated for the selected position. When the normal delegation position is selected the earnings are regularly updated and one can see how much the position has generated. With the `Claim` button one can anytime claim the rewards generated until now. If one wants to close the position at all, after claiming the rewards, the user has to execute a second transaction for undelegating the position. Just so you know, if you close the position (undelegate), the rewards will automatically be claimed. So, no second transaction is required in this case.\
  Rewards for the vesting positions are calculated once the positions have ended. When this happens, the maturing period will activate and the earnings will regularly be updated and one can freely decide how and when to claim them.

- To undelegate, simply click the `Undelegate` button. A modal will appear where you can see the available LYDRA, the amount delegated in this position, and a field to enter the amount you wish to undelegate. You can also use the `MAX` button to fill in the full delegated amount automatically. Note that you must have the same amount of Lydra available in your wallet to proceed with the undelegation.

- Below, you'll see the amount of HYDRA you'll receive and the approximate network fee for the transaction. If the position is vested and still active, a warning message will appear, indicating that the vesting period isn't over yet. It will also provide an approximate calculation of the penalty for undelegating prematurely, and a reminder that all rewards will be forfeited.

- The process for pending transactions and confirmations remains the same. Once the transaction is confirmed, the table will be updated to reflect the remaining staked amount, if any."

- When a position is undelegated, the system will register a withdrawal on the blockchain and the user will have to wait for the withdrawal period, which currently is 1 epoch. Under the Delegation Info sections, there is a table that will show all available withdrawables once the period of 1 epoch has passed. In the `Actions` section, the user will be able to `Withdraw` the amount at any time.
