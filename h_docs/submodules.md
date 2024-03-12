Polygon Edge uses two submodules one of which is the core contracts repo.

We are working on a fork of the core contracts, so the submodule is update the work with our custom version.

Commands used for the module update:

```
git submodule set-url core-contracts https://github.com/Hydra-Chain/hydragon-core-contracts.git
```

```
git submodule update --init  --remote ./core-contracts
```

Install dependencies

```
cd core-contracts
```

```
npm i
```

Compile contracts

```
npx hardhat compile
```

Run tests to ensure everything is working properly

```
npm run test:hydra
```

## How to generate core-contracts binaries:

After successful init and update of the core-contracts submodule run the following:

```
cd ..
go run ./consensus/polybft/contractsapi/artifacts-gen/main.go
go run ./consensus/polybft/contractsapi/bindings-gen/main.go
```
