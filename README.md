# Staking Indexer

The staking indexer is a tool that extracts BTC staking relevant data from the
Bitcoin blockchain, ensures that it follows the pre-requisites for a valid
staking transaction, and determines whether the transaction should be active or
not. All valid staking transactions are transformed into a structured form,
stored in a database, and published as events in a RabbitMQ messaging queue for
consumption by consumers. The staking indexer is the enforcer of the Bitcoin
Staking protocol and serves as the ground truth for the Bitcoin Staking system.

## Features

1. Polling BTC blocks data from a specified height in an ongoing manner. The 
   poller ensures that all the output blocks have at least `N` confirmations 
   where `N` is a configurable value, which should be large enough so that 
   the chance of the output blocks being forked is enormously low, e.g., 
   greater than or equal to `6` in Bitcoin mainnet. In case of major reorg,
   the indexer will terminate and should manually bootstrap from a clean DB.
2. Extracting transaction data for staking, unbonding, and withdrawal. These 
   transactions are verified and compared against the system parameters to 
   identify whether they are active, inactive due to staking cap overflow, 
   or invalid. The details of the protocol for verifying and activating 
   transactions can be found [here](./doc/staking.md).
3. Calculating confirmed and unconfirmed TVL (total value locked) based on
   observed transactions.
4. Storing the extracted transaction data and system state in a database. The 
   details can be found [here](./doc/state).
5. Pushing staking, unbonding, withdrawal events, and TVL calculation 
   results to the message queues. 
   A reference implementation based on [rabbitmq](https://www.rabbitmq.com/) 
   is provided. The definition of each type of events can be found [here](./doc/events.md).
   Our [API service](https://github.com/babylonlabs-io/staking-api-service)
   exhibits how these events are utilized and presented.
6. Monitoring the status of the service through [Prometheus metrics](./doc/metrics.md).
7. Exporting staking transactions from the indexer store to a CSV file.

## Usage

### 1. Setup bitcoind node

The staking indexer relies on `bitcoind` as backend. Follow this [guide](./doc/bitcoind_setup.md)
to set up a `bitcoind` node.

### 2. Install

Clone the repository to your local machine from Github:

```bash
git clone https://github.com/babylonlabs-io/staking-indexer.git
```

Install the `sid` daemon binary by running:

```bash
cd staking-indexer # cd into the project directory
make install
```

### 3. Configuration

To initiate the program with default config file, run:

```bash
sid init
```

This will create a `sid.conf` file in the default home directory. The 
default home directories for different operating systems are:

- **MacOS** `~/Users/<username>/Library/Application Support/Sid`
- **Linux** `~/.Sid`
- **Windows** `C:\Users\<username>\AppData\Local\Sid`

Use the `--home` flag to specify the home directory and use the `--force` to 
overwrite the existing config file.

### 4. Run the Staking Indexer

To run the staking indexer, we need to prepare a `global-params.json` file
which defines all the global params that are used across the BTC staking
system. The indexer needs it to parse staking transaction data.
The definition of global params can be found [here](./doc/staking.md#staking-parameters).
An example of the global params can be found in [test-params.json](./itest/test-params.json).
The program reads the file from the home directory by default. The user can
specify the file path using the `--params-path` flag.

To run the staking indexer from a specific height, run:

```bash
sid start --start-height <start-height>
```

If the `--start-height` is not specified, the indexer will retrieve the 
start height first from the database which saves the `last_processed_height`. 
If the database is empty, the start height will be retrieved from the earliest
`activation_height` defined in the global parameters file.
The earliest `activation_height` is a height before which no staking transactions
have been included in.
Note that if the database is empty, the indexer will strictly start from the
earliest `activation_height`. If the database is not empty, the user can specify
a height that is not higher than `last_processed_height + 1` via `--start-height`.
This is to ensure that no staking data will be missed.

### 5. Exporting staking transactions

We can export the indexed staking transactions via the command:

```bash
sid export --start-height <start-height> --end-height <end-height> --output transactions.csv
```

```txt
Transaction Hash,Staking Output Index,Inclusion Height,Staker Public Key,Staking Time,Finality Provider Public Key,Is Overflow,Staking Value
1b42ce46130a1d4b3bdd56b5cb325976851af1ab76951565aa4858c7d16dad00,0,864790,139f4e3ec192e83b9c6789ff644261b8fa5d7b716d1813bee744e3472f264d99,64000,fa7496f63a857d894aa393767325bf6f84560e9141f4ec54496c50f546f48bfb,true,1905000
3ddfb76b9971b786fc798a98f8fc5edc42a074c47ef28df812a389c16536b401,0,864790,b18ac73a57e6d3413284d1c91c14744464d71f19397c8ab053bc99c1ed96cafe,64000,bb0bceda25d82f10a69feca9c076d85f61d750c9a481b8105d8389325538fdd1,true,500000
36f7042e0eec9f3364ed481acf203a6644eb25e83942582af207f695eb0ebe04,0,864790,33fe5ec5f928a5320867353abb754b0f20f2ccaf4eac3373abbd957ec8007419,64000,fa7496f63a857d894aa393767325bf6f84560e9141f4ec54496c50f546f48bfb,true,555000
2b87c266121e543494b2c3a5f06855475ccebcfc66c552f3e2bd448832ff9205,0,864790,e8ef702fab83e6d022bc1e5c55d9f939ff0176c9d4e5269f9b0518d852e44ac8,64000,fa7496f63a857d894aa393767325bf6f84560e9141f4ec54496c50f546f48bfb,true,500000
```

### Tests

Run unit tests:

```bash
make test
```

Run e2e tests:

```bash
make test-e2e
```

This will initiate docker containers for both a `bitcoind` node running in the 
signet mode and a `rabbitmq` instance.
