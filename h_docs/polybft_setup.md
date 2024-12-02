# Launch PolyBFT chain

There is no official docs on polybft yet, so I will write my findings here.
**EDIT:** there is already official docs but we segnificantly modify the way our consensus mechanism work because we don't have
root chain in our setup. So I will continue describing the way to setup a chain in this file.

## Local setup

This setup is mainly used for development and testing purposes.

### Local Setup for version 0.9.x

#### Initial chain setup

I am describing our custom process, because it is different.

1. Generate secrets

```
./hydra secrets init --chain-id 8844 --data-dir test-chain-1 --insecure /
./hydra secrets init --chain-id 8844 --data-dir test-chain-2 --insecure /
./hydra secrets init --chain-id 8844 --data-dir test-chain-3 --insecure /
./hydra secrets init --chain-id 8844 --data-dir test-chain-4 --insecure /
./hydra secrets init --chain-id 8844 --data-dir test-chain-5 --insecure
```

1. Generate third party secrets (CoinGecko). You can retrieve free keys from the corresponding website.

```
./hydra secrets generate --type local --name node --extra "coingecko-api-key=<key>"
```

3. Generate genesis file

We need to set native token to be mintable, so we can premine balances to different addresses. Keep in mind that the validators need some premined coins, so, add it before generating the genesis. They are needed in order validators to feed the Price Oracle (executing transactions). Use the command below to generate the file. Make sure to update the proxy contracts admin and governance flags according to your requirements.

```
./hydra genesis --block-gas-limit 10000000 --epoch-size 10 --validators-path ./ --validators-prefix test-chain- --consensus polybft --native-token-config Hydra:HDR:18:true:0x211881Bb4893dd733825A2D97e48bFc38cc70a0c --premine 0x211881Bb4893dd733825A2D97e48bFc38cc70a0c:70000000000000000000000 --premine 0xdC3312E368A178e24850C6dAC169646c5fD14b93:30000000000000000000000 --proxy-contracts-admin 0x211881Bb4893dd733825A2D97e48bFc38cc70a0c --governance <governance-address-here> --chain-id 8844
```

4. Run the chain

```
./hydra server --data-dir ./test-chain-1 --chain genesis.json --grpc-address :5001 --libp2p :30301 --jsonrpc :10001 --log-level DEBUG --log-to ./log

./hydra server --data-dir ./test-chain-2 --chain genesis.json --grpc-address :5002 --libp2p :30302 --jsonrpc :10002 --log-level DEBUG --log-to ./log-2

./hydra server --data-dir ./test-chain-3 --chain genesis.json --grpc-address :5003 --libp2p :30303 --jsonrpc :10003 --log-level DEBUG --log-to ./log-3

./hydra server --data-dir ./test-chain-4 --chain genesis.json --grpc-address :5004 --libp2p :30304 --jsonrpc :10004 --log-level DEBUG --log-to ./log-4

./hydra server --data-dir ./test-chain-5 --chain genesis.json --grpc-address :5005 --libp2p :30305 --jsonrpc :10005 --log-level DEBUG --log-to ./log-5
```

#### Add more validators

1. Generate new account (secrets):

```
./hydra secrets init --chain-id 8844 --data-dir test-add-chain-1 --insecure
```

In order to output private and public keys, use the following commands:
```
./hydra secrets output-private --data-dir ./test-add-chain-1 --insecure
```

for the secret keys, and the following for the public keys, including the Node ID:
```
./hydra secrets output-public --data-dir ./test-add-chain-1 --insecure
```

2. Use the governer (first validator by default) to whitelist the new account

```
./hydra hydragon whitelist-validator --data-dir ./test-chain-1 --address <public address> --jsonrpc http://127.0.0.1:10001 --insecure
```

3. Register account

--data-dir is the direcotry of the freshly created secrets  
Stake tx is made in this step as well

```
./hydra hydragon register-validator --data-dir ./test-add-chain-1 --stake 15000000000000000000000  --commission 10 --jsonrpc http://127.0.0.1:10001 --insecure
```

4. Run new validator

```
./hydra server --data-dir ./test-add-chain-1 --chain genesis.json --grpc-address :5006 --libp2p :30306 --jsonrpc :10006 --log-level DEBUG --log-to ./log-6
```

5. Staking

After registering, you can increase your stake at any time. Additionally, if you have previously unstaked, stopped your validator, and want to resume validating, you don’t need to register again. You can simply start validating again by adding a stake using the following command:

```
./hydra hydragon stake --data-dir ./test-add-chain-1 --self true --amount 10000000000000000000000 --jsonrpc http://127.0.0.1:10006 --insecure
```

To stake a vested position, use the same command but include the additional vesting flag:

```
./hydra hydragon stake --data-dir ./test-add-chain-1 --self true --amount 10000000000000000000000 --vesting-period 52 --jsonrpc http://127.0.0.1:10006 --insecure
```

**Note:** The specified amount will be added to your existing staked amount, if applicable.

6. Update the commission of the validator that will taken from the delegators' rewards.

Here is the command to use for initializing the new commission:

```
./hydra hydragon commission --data-dir ./test-add-chain-1 --commission 20 --jsonrpc http://127.0.0.1:10006 --insecure
```

Then, we have a 15-day waiting period before being able to apply the commission. The apply can be done by using the `--apply` flag:

```
./hydra hydragon commission --data-dir ./test-add-chain-1 --apply true --jsonrpc http://127.0.0.1:10006 --insecure
```

7. The new validator will join the consensus in the next epoch. Then, after each epoch, rewards will be generated and can be claimed with the following command:

```
./hydra hydragon claim-rewards --data-dir ./test-add-chain-1 --jsonrpc http://127.0.0.1:10006 --insecure
```

Additionally, during rewards distribution, if a validator has delegators, an additional reward based on the set commission (if greater than 0) will be generated when delegators claim their rewards. The validator can claim these commission rewards by executing the following command:

```
./hydra hydragon commission --data-dir ./test-add-chain-1 --claim true --jsonrpc http://127.0.0.1:10006 --insecure
```

8. Re-activating the validator, if a ban initiation took place:

```
./hydra hydragon terminate-ban --data-dir ./test-add-chain-1 --jsonrpc http://127.0.0.1:10001 --insecure
```

9. If the validator was banned, then withdraw the funds left by executing:

```
./hydra hydragon withdraw --banned --data-dir ./test-add-chain-1 --jsonrpc http://127.0.0.1:10001 --insecure
```

### LEGACY local setup

1. Generate secrets

```
./hydra secrets init --data-dir test-chain-1
./hydra secrets init --data-dir test-chain-2

```

2. Create manifest file
  This is the first version of edge that needs a manifest file. It contains information about the initial validators.

```
./hydra manifest --validators-prefix test-chain-
```

3. Generate genesis file

```
./hydra genesis --consensus polybft --ibft-validators-prefix-path test-chain-
```

4. Run the chain

```
./hydra server --data-dir ./test-chain-1 --chain genesis.json --grpc-address :10000 --libp2p :10001 --jsonrpc :10002 --log-level=DEBUG
```

## Devnet node setup

1. Pull hydrag-devnet image from DockerHub

```
docker pull rsantev/hydrag-devnet:latest
```

2. If you already have generated secrets, add them as environment variables

Go to the next step in case you haven't already generated secrets

Create an env.list file with the following data:

```
BLS_KEY=
KEY=<account eth-like private key>
P2P_KEY=
SIG=<BLS signature>
```

All above variables can be found in the folder generated by the secrets command execution. (check local setup)

Pass the env.list file when running the container:

```
--env-file env.list image_name
```

3. Run the container with a proper configuration

We do the following configuration:

- Map port 1478 of the container to port 1478 of the host machine
- Mounts the directory /path/on/host from the host machine to /app/node inside the container.
- Add the following command:
  ```
  server --data-dir ./node --chain genesis.json --grpc-address 127.0.0.1:9632 --libp2p 0.0.0.0:1478 --jsonrpc 0.0.0.0:8545 --prometheus 0.0.0.0:5001 --log-level DEBUG json-rpc-block-range-limit 0
  ```

  ```
  docker run -p 1478:1478 -v /path/on/host:/app/node <optional env.list path> hydrag-devnet
  ```

If secrets env variables are not added, new secrets will be automatically created for you.

4. Prepare account to be a validator

You need to request two things from Hydra's team:

- Fulfill your account with enough Hydra
- Whitelist your account to be able to participate as a validator

Open the container shell:

```
docker exec -it <container name or ID; check it with docker ps> /bin/bash
```

Check your public data with the following command:

```
./hydra secrets output-public --data-dir ./node --insecure
```

You need the following value:

**Validator Address = 0x...**

Send it to Hydra's team.

5. Register account as validator and stake

After Hydra's team confirms you are whitelisted you have to register your account as a validator and stake a given amount.
In the container's shell execute:

```
./hydra hydragon register-validator --data-dir ./node --stake 15000000000000000000000 --commission 10 --jsonrpc http://localhost:8545
```

The above command both register the validator and stakes the specified amount.

Use the following command in case you want to execute the stake operation only:

```
./hydra hydragon stake --data-dir ./node --self true --amount 15000000000000000000000 --jsonrpc http://localhost:8545
```

Congratulations! You are now a Hydra Chain validator!

6. After becoming a validator, you can set the desired commission that will be deducted from the delegators' rewards.
Additionally, you can update the commission if you need to.

```
./hydra hydragon commission --data-dir ./node --commission 10 --jsonrpc http://localhost:8545
```

### Admin actions for devnet node setup

#### Send Hydra

1. Extract private key from Node

In the container's shell execute:

```
cat /app/node/consensus/validator.key
```

Setup any compatible wallet and execute the transfer from there.

2. Whitelist account

In the container's shell execute:

```
./hydra hydragon whitelist-validator --data-dir ./node --address <provided address> --jsonrpc http://localhost:8545
```
