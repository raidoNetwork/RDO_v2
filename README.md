# Raido Chain

RaidoChain node implementation.

## Building instructions

Building `raido` requires Go (version 1.17 or higher), a C compiler (gcc version 12.0.5 or higher) and MySQL Database Server (version 8.0 or higher).
When all dependencies are installed, run:
```bash
$ make raido
```

or, to build another utilities use:
```bash
$ make all
```

## Running `raido`

```bash
$ raido
```
This command will start raido node with default parameters.

### Logging
Logs are output to the path set with flag `-log-file`. If flag is not specified logs are output to `os.Stdout`.

### Recommended usage
```bash
$ raido --config-file=config.yaml --chain-config-file=net.yaml
```

The `--config-file` flag will configure node to use config file params instead of flags.

*Note: Сonfig file params have higher priority than given flags!*

Config file can take all flags from the list below as params.

### Node flags

| <div style="width:150px;">**Flag** </div> |  **Desciprtion** | 
|--------------------------|-------------------------------|
| `rpc-host` | Host on which the RPC server should listen. |
| `rpc-port` | RPC port exposed by a rdo node.             |  
| `grpc-gateway-host` | The host on which the gateway server runs on. |
| `grpc-gateway-port` | The port on which the gateway server runs on. |
| `grpc-gateway-corsdomain` | Comma separated list of domains from which to accept cross origin requests (browser enforced). This flag has no effect if not used with --grpc-gateway-port. |
| `p2p-host` | The host on which p2p service should listen. |
| `p2p-port` | The port on which p2p service runs on. |
| `p2p-bootstrap-nodes` | List of P2P nodes addresses for initial connections. |
| `enable-metrics` | Enables Prometheus monitoring service. | 
| `metrics-host` | The host on which metrics endpoint should runs on. |
| `metrics-port` | The port on which metrics endpoint should runs on. |
| `verbosity` | Logging verbosity (trace, debug, info=default, warn, error, fatal, panic). |
| `sql-cfg` | Config file path with MySQL host, port, user and password. |
| `datadir` | Data directory for the databases and keystore. |
| `log-file` | Specify log file name, relative or absolute. |

### Network settings

`--chain-config-file` specifies path to the network config file with the following params:

|     **Param**     |  **Desciprtion** | 
|--------------------------|-------------------------------|
| `SLOT_TIME` | Block creation time in seconds. |
| `REWARD_BASE` | Fixed reward per block for stakers in roi. |
| `MINIMAL_FEE` | Minimal price per byte for transaction in roi. |
| `STAKE_SLOT_UNIT` | Amount needed to fill stake slot in RDO. |
| `BLOCK_SIZE` | Block maximum size in bytes. |
| `VALIDATOR_REGISTRY_LIMIT` | Validator slots count. |
| `GENESIS_PATH` | Path to the Genesis json. |

## Genesis block

To create special Genesis block use structure below:
```json
 {
  "timestamp": 121232131, 
  "hash": "Genesis hash",
  "outputs": {
      "0xaddress1": 123456789,
      "0xaddress2": 12345
    }
}
```

## Web3 API
`Raido node` has built-in support for a JSON-RPC based APIs to interact with it. This can be exposed **only** via HTTP.
Full list of `raido node` API methods is presented in Swagger Docs.

### Signing and verifying data
Signing any data and signature verification with Node.js requires npm packages `keccak` and `secp256k1` to be installed.
Example:
```javascript
const createKeccakHash = require("keccak");
const {ecdsaSign, ecdsaRecover} = require("secp256k1");

function createKeccak256(){
    let createHash = function(){
        return createKeccakHash("keccak256");
    }

    return function (data) {
        let hash = createHash();
        hash.update(data);
        return Buffer.from(hash.digest());
    };
}

const keccak256 = createKeccak256()

function hexToBytes(hex) {
    hex = hex.toString(16);

    if (!isHexStrict(hex)) {
        throw new Error('Given value "'+ hex +'" is not a valid hex string.');
    }

    hex = hex.replace(/^0x/i,'');

    let bytes = [];
    for (let c = 0; c < hex.length; c += 2){
        bytes.push(parseInt(hex.substr(c, 2), 16));
    }

    return bytes;
}

function isHexStrict(hex) {
    return ((typeof hex === 'string' || typeof hex === 'number') && /^(-)?0x[0-9a-f]*$/i.test(hex));
}

function createSignDigest(data){
    let dataBuffer = keccak256(Buffer.from(hexToBytes(data)));
    let preBuffer = Buffer.from("\x15RaidoSignedData\n");
    let dgstBuffer = Buffer.concat([preBuffer, dataBuffer]);

    return keccak256(dgstBuffer);
}

function signTx(hash, privateKey){
    if (privateKey.startsWith('0x')) {
        privateKey = privateKey.substring(2);
    }
    // 64 hex characters
    if (privateKey.length !== 64) {
        throw new Error("Private key must be 32 bytes long");
    }

    let dgst = createSignDigest(hash);
    let {signature, recid} = ecdsaSign(dgst, Buffer.from(privateKey, 'hex'));

    return Buffer.concat([Buffer.from(signature), Buffer.from([recid])]);
}

function verifySign(signature, hash, address){
    let sigBuf = Buffer.from(signature, 'hex');
    const recid = sigBuf[sigBuf.length - 1]; // set recid to the 0 or 1

    sigBuf = sigBuf.slice(0, -1);

    const dataHash = createSignDigest(hash);
    const pubKeyArr = ecdsaRecover(Uint8Array.from(sigBuf), recid, Uint8Array.from(dataHash), false);
    const pubKeyHash = keccak256(Buffer.from(pubKeyArr.slice(1))).slice(12);
    const pubkeyHex = "0x" + pubKeyHash.toString('hex');

    return pubkeyHex == address;
}

const txHash = "0x4ad3d34bd3b337c26a10e73b4db0d0663835c32663046bfe4cba477fde84a44b";
const privateKey = "0x2e582a40675d772f0229e56ce99e1fe237cad7f919302b7c77870a5f8f9ab"; // not valid private key
const address = "0xe3655ecb76351e144bff15058e0b56c1539082a6";

const sign = signTx(txHash, privateKey);
const signHash = sign.toString('hex');

console.log("Transaction signature:", signHash);

const isSignValid = verifySign(sign, txHash, address);

console.log("Is sign valid: ", isSignValid);
```
