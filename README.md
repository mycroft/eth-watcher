eth-watcher
===========

## Introduction

`eth-watcher` is a simple API server that will allow to:

- create new Ethereum addresses;
- retrieve ETH or erc20 balance for any ETH address;
- notification system;
- send ETH coins or erc20 tokens from an address to another.

## Compilation & set-up

`eth-watcher` compiles with golang >= 1.10. It has a few dependencies, like `gorilla/mux` & `gorilla/websocket`, `go-sql-driver/mysql` and of course `ethereum/go-ethereum`.

```shell
$ git clone https://gitlab.mkz.me/mycroft/eth-watcher
$ cd eth-watcher
$ go get -d -v
$ go build
```

Before running, you need to create a configuration file. A sample file, `config.ini.sample` is provided in the repository. Please configure the RPC & Websocket hosts of geth to make `eth-watcher` able to connect.

```shell
$ cp config.ini.sample config.ini
$ cat config.ini
[network]
rpc_host = 10.0.0.7:8545
websocket_host = 10.0.0.7:8546

[db]
protocol = tcp
host = 172.17.0.2
name = eth
user = eth_user
pass = eth_pass
```

Once compiled & configured, you just need to create sql tables. `eth-watcher` is able to create them by itself:

```shell
$ ./eth-watcher -init
```

## Running

To run the daemon, just start it by running it:

```shell
$ ./eth-watcher
```


## API Endpoints

### Create new Ethereum address

Create a new Ethereum key pair and store it in database.

#### URL

  /createAddress

#### Method

  POST

#### URL Params

   **Optional:**

   `with_private=true`
   If set, returns the private key in the response.

#### Success response:

  * **Code:** 200<br>
    **Content:** `{"response":{"address":"5a8152656ca1824ea43e6d045f3c884bf4c93f65"},"result":"success"}`

#### Error response:

  * **Code:** 500<br>
    **Content:** `{"response":{"error":"Could not save newly created key: Error 1146: Table 'eth.eth_keys' doesn't exist"},"result":"failure"}`

#### Sample

```shell
$ curl -s -X POST "http://localhost:8080/createAddress" | python -mjson.tool 
{
    "response": {
        "address": "64341b18d064eb623c11cb9a3c59198b60bad668"
    },
    "result": "success"
}

$ curl -s -X POST "http://localhost:8080/createAddress?with_private=true" | python -mjson.tool
{
    "response": {
        "address": "021c2756c8e1a61cf5a46819c2a51efc1478e605",
        "private": "192f072c51d7ee0fc498271668feecbd41010e4d4402fff7c61dbf4515afeadd"
    },
    "result": "success"
}
```

### Register an existing Ethereum address in database

Save in database an external Ethereum address (with or without private key)

#### URL

  /registerAddress

#### Method

  POST

#### Data Params

  **Mandatory:**

  `address=[address]`: The address to register

  **Optional:**

  `private=[private]`: The private key to associate to given address key

#### Success response:

  * **Code:** 200<br>
    **Content:** `{"response":{"message":"Address saved in database"},"result":"success"}`

#### Error response:

  * **Code:** 500<br>
    **Content:** `{"response":{"error":"Could not save newly created key: Error 1146: Table 'eth.eth_keys' doesn't exist"},"result":"failure"}`

#### Sample

```shell
$ curl -q -X POST -d address=75e59402d6f5ac5ea875ac4d63d9012a43777119 -d private=952782d2bc3e9c8802e0c2c2e282da5816d297b5c7b6d5120e99826942def3fa "http://localhost:8080/registerAddress"
{"response":{"message":"Address saved in database"},"result":"success"}
```


### Retrieve Ethereum balance

Returns the Ethereum coin (ETH) balance or the erc20 token balance (if `contract` parameter is set).

#### URL

  /getBalance

#### Method

  GET

#### URL Params

  **Mandatory:**

  `address=[address]`

  **Optional:**

  `contract=[address]`
  If set, will return the erc20 token balance. Unset, returns ETH balance.

#### Success response:

  * **Code:** 200<br>
    **Content:** `{"response":{"balance":"6.0999900000"},"result":"success"}`

#### Error response:

  * **Code:** 500<br>
    **Content:** `{"response":{"error":"Could not retrieve ethereum balance: Post http://10.0.0.7:8544: dial tcp 10.0.0.7:8544: connect: connection refused"},"result":"failure"}`

  * **Code:** 500<br>
    **Content:** `{"response":{"error":"Could not retrieve ethereum balance: Failed to retrieve balance for token: no contract code at given address"},"result":"failure"}`

#### Samples:

```shell
$ export ADDRESS=0x85e31428748622432ab6c13d4a3a5319f0a67186
$ export CONTRACT=0xa3C9336a549fD2d809B34c421257d1d8B94603c8

$ curl "http://localhost:8080/getBalance?address=$ADDRESS"
{"response":{"balance":"6.2229900000"},"result":"success"}

$ curl "http://localhost:8080/getBalance?address=$ADDRESS&contract=$CONTRACT"
{"response":{"balance":"13"},"result":"success"}
```

### Send Ethereum coin

Send coins using a private key to an address

#### URL

  /sendEth

#### Method

  POST

#### Data Params

  **Mandatory:**

  `private=[private]` The private key to use to send coins

  `address=[address]` The address key to send coins to

  `amount=[amount]` Amount of coins (in ETH)

#### Success response:

  * **Code:** 200<br>
    **Content:** `{"response":{"txhash":"0x151d37bdcb8afc2427ff3d4eea8e99735313c272e1498c2f6dbd793e804e11af"},"result":"success"}`

#### Error response:

  * **Code:** 500<br>
    **Content:** `{"response":{"error":"Could not send Ethereum coin: Send tx error: known transaction: 151d37bdcb8afc2427ff3d4eea8e99735313c272e1498c2f6dbd793e804e11af"},"result":"failure"}`

  * **Code:** 500<br>
    **Content:** `{"response":{"error":"Could not send Ethereum coin: Send tx error: replacement transaction underpriced"},"result":"failure"}`

  * **Code:** 400<br>
    **Content:** `{"response":{"error":"Missing 'private' field"},"result":"failure"}`

#### Samples:

```shell
$ export FROM_PRIVATE=f0abab15e15b43826a746c89ceb740ad28b0fc683475d696f5c17d924cdd9294
$ export TO_ADDRESS=0x85e31428748622432ab6c13d4a3a5319f0a67186
$ export AMOUNT=0.123

$ curl -X POST -d private=$FROM_PRIVATE -d address=$TO_ADDRESS -d amount=$AMOUNT "http://localhost:8080/sendEth"
{"response":{"txhash":"0xeb85126d4a8266616115aa7fb9c5759b4cf971588aed427de377a328aa169c2a"},"result":"success"}
```

In Ethereum console:

```javascript
> web3.eth.getTransaction("0xeb85126d4a8266616115aa7fb9c5759b4cf971588aed427de377a328aa169c2a")
{
  blockHash: "0x0000000000000000000000000000000000000000000000000000000000000000",
  blockNumber: null,
  from: "0xc97ec1b4bf2b0106f951e113690b194289037d52",
  gas: 60000,
  gasPrice: 0,
  hash: "0xeb85126d4a8266616115aa7fb9c5759b4cf971588aed427de377a328aa169c2a",
  input: "0x",
  nonce: 12,
  r: "0xba2b6d6c6ed184772e07ab49f09fa84aca22eb94af22ded82faf09f212d88779",
  s: "0x919154a5658d30322e80b57c35ed481358a70ec147b514516d93b141f6d89b5",
  to: "0x85e31428748622432ab6c13d4a3a5319f0a67186",
  transactionIndex: 0,
  v: "0x1c",
  value: 123000000000000000
}
```

### Send ERC20 token

Send ERC20 token using a private key and contract address to another Ethereum address

#### URL

  /sendErc20

#### Method

  POST

#### Data Params

  **Mandatory:**

  `contract=[contract]` The contract address to use

  `private=[private]` The private key to use to send coins; At least a `private` or an `address_from` is required

  `address_from=[address_from]` The address to use to send coins, retrieving private key from database; At least a `private` or an `address_from` is required

  `address=[address]` The address key to send coins to

  `amount=[amount]` Amount of coins (in ETH)

#### Success response:

  * **Code:** 200<br>
    **Content:** `{"response":{"txhash":"0xf2accd8638975992b14007a71e91cf34631b03d67aa0b265172c6001993272b4"},"result":"success"}`

#### Error response:

  * **Code:** 500<br>
    **Content:** `{"response":{"error":"Could not send ERC20 token: no contract code at given address"},"result":"failure"}`

#### Samples:

```shell
$ export CONTRACT=0xa3C9336a549fD2d809B34c421257d1d8B94603c8
$ export FROM_PRIVATE=f0abab15e15b43826a746c89ceb740ad28b0fc683475d696f5c17d924cdd9294
$ export TO_ADDRESS=0x85e31428748622432ab6c13d4a3a5319f0a67186
$ export AMOUNT=2

$ curl -X POST -d contract=$CONTRACT -d address=$ADDRESS -d private=$FROM_PRIVATE -d address=$TO_ADDRESS -d amount=2 http://localhost:8080/sendErc20

{"response":{"txhash":"0xf2accd8638975992b14007a71e91cf34631b03d67aa0b265172c6001993272b4"},"result":"success"}
```

In Ethereum console:

```javascript
> web3.eth.getTransaction("0xf2accd8638975992b14007a71e91cf34631b03d67aa0b265172c6001993272b4")
{
  blockHash: "0x0000000000000000000000000000000000000000000000000000000000000000",
  blockNumber: null,
  from: "0xc97ec1b4bf2b0106f951e113690b194289037d52",
  gas: 36240,
  gasPrice: 18000000000,
  hash: "0xf2accd8638975992b14007a71e91cf34631b03d67aa0b265172c6001993272b4",
  input: "0xa9059cbb00000000000000000000000085e31428748622432ab6c13d4a3a5319f0a671860000000000000000000000000000000000000000000000000000000000000002",
  nonce: 13,
  r: "0x1bc05df811bf54c422347d71f783509209426468fed36169637b2c20f41c5ed5",
  s: "0x3d0a826bb3866b932b6dff5e85c288e65cf6390f454d9640c39decf7a158968f",
  to: "0xa3c9336a549fd2d809b34c421257d1d8b94603c8",
  transactionIndex: 0,
  v: "0x1b",
  value: 0
}
```

### Get notifications

#### URL

  /getNotifications

#### Method

  GET

#### URL Params

   **Optional:**

   `remove=true`
   If set, remove from the database the records.

#### Samples:

```shell
$ curl -s http://localhost:8080/getNotifications|python -mjson.tool
{
    "response": [
        {
            "AddressFrom": "C97eC1b4bF2b0106f951E113690B194289037D52",
            "AddressTo": "5A8152656cA1824ea43e6D045F3C884Bf4c93F65",
            "Amount": 11000000000000000,
            "ContractAddress": "",
            "IsPending": true,
            "MessageType": 0,
            "TxHash": "521086c8b8334325477ce2a80ddcb1e69176b8f74736b0300541d0f4593025a2"
        },
        {
            "AddressFrom": "C97eC1b4bF2b0106f951E113690B194289037D52",
            "AddressTo": "5A8152656cA1824ea43e6D045F3C884Bf4c93F65",
            "Amount": 11000000000000000,
            "ContractAddress": "",
            "IsPending": false,
            "MessageType": 0,
            "TxHash": "521086c8b8334325477ce2a80ddcb1e69176b8f74736b0300541d0f4593025a2"
        }
}
```


## Technical notes

### Tests

`eth-watcher` was tested on both private & public Ethereum network, using the following command line:

```shell
$ /opt/ethereum/go-ethereum/build/bin/geth --rpc --rpcaddr 0.0.0.0 --ws --wsaddr 0.0.0.0 --wsorigins * --datadir /home/coins/ethereum console
```

### token.abi file

This file is mandatory to have an erc20 skeleton token interface and easily create send orders (erc20 function addresses never change).
To generate it, we used method on [Native DApps: Go bindings to Ethereum contracts](https://github.com/ethereum/go-ethereum/wiki/Native-DApps:-Go-bindings-to-Ethereum-contracts). Using the sample file, we just created token.go doing:

```shell
$ abigen --abi token.abi --pkg main --type Token --out token.go
```

### Database schema

It is created using `./eth-watcher -init`

```sql
CREATE TABLE eth_keys(
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    address VARCHAR(40),
    private VARCHAR(64)
);

CREATE INDEX eth_keys_address_idx ON eth_keys(address);

CREATE TABLE notifications(
    id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    address_from     VARCHAR(40),
    address_to       VARCHAR(40),
    address_contract VARCHAR(40),
    amount           VARCHAR(32),
    is_pending       BOOLEAN NOT NULL DEFAULT false,
    tx_hash          VARCHAR(64),
    created_at       DATETIME DEFAULT NOW()
);

CREATE TABLE settings(
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(32) UNIQUE,
    value VARCHAR(64)
);
```