# Ethereum Native Media Transaction Platform — Project Analysis & Implementation Outline

---

## 1. Executive Summary & Core Objective

### Problem Statement
Standard decentralized media transactions (such as NFT marketplaces and digital artwork sales) rely on external, decoupled storage solutions such as IPFS, Arweave, or centralized S3 buckets. While Ethereum handles payments and token ownership natively, transferring the underlying media asset usually requires off-chain trust assumptions, pinning services, and third-party gateways.

### Project Objective
The goal of this project is to create an **autonomous, 2-way media/artwork transaction mechanism built natively into Ethereum (Geth)** without external dependencies like IPFS.
The transaction consists of two coordinated parts:
1. **On-Chain Settlement**: Payment in ETH, sale posting with cryptographic signatures, and delivery receipt confirmation handled by an Ethereum smart contract (`ArtworkMarketplace.sol`).
2. **Native In-Node Asset Persistence**: Ephemeral storage of the media files directly within Ethereum execution client nodes (`go-ethereum`) through an **Asset Pool** subsystem (analogous to the TxPool/BlobPool).

### Key Architectural Tenets
- **Asset Pool**: An ephemeral node-level storage layer holding media files during an active transaction window.
- **48-Hour Purchaser Window**: Nodes persist the media file for a default retention period of 48 hours (`CreatedAt + RetentionWindow`), providing the purchaser ample time to fetch and verify the original file before the nodes prune the data.
- **Node-Level Cryptographic Integrity**: When a creator submits media, the node hashes the file using Keccak-256 and verifies the creator's secp256k1 signature. Spoofed or unverified payloads are rejected at the node boundary.
- **On-Chain Designated Purchaser Support**: Creators can list artworks publicly or designate a specific recipient address who alone is permitted to execute the purchase.
- **Automatic Pruning**: A background eviction worker periodically sweeps node storage to remove expired or fully acknowledged media assets, keeping node operational and disk costs minimal.

---

## 2. Directory Structure & Current Component Map

```
eth-media-transaction/
├── README.md                           # Original project requirements & architecture notes
├── asset.md                            # Comprehensive analysis & execution outline (this document)
├── artwork-market-platform/            # Application layer & orchestrations
│   ├── contracts/                      # Smart contract subsystem (Foundry)
│   │   ├── foundry.toml                # Foundry configuration
│   │   ├── src/
│   │   │   └── ArtworkMarketplace.sol  # Marketplace smart contract with EIP-191 signature recovery
│   │   └── test/
│   │       └── ArtworkMarketplace.t.sol# 4 Foundry unit tests (all passing)
│   ├── genesis.json                    # Custom testnet genesis with pre-funded accounts
│   ├── scripts/                        # Automation & deployment scripts
│   │   ├── generate_genesis.py         # Genesis generator using geth --dev dumpgenesis
│   │   ├── deploy_contracts.py         # Automates forge create & generates web/config.json
│   │   └── seed_sample_art.py          # Seeds 'Aurora Genesis #001' into Asset Pool & contract
│   ├── start.sh                        # All-in-one bootstrap script (builds geth, starts testnet, deploys, serves UI)
│   └── web/                            # Interactive Web Interface (Vanilla JS + Ethers v6)
│       ├── index.html                  # Multi-tab UI (Creator Studio, Gallery, Vault, Inspector)
│       ├── style.css                   # Modern dark-mode styling
│       ├── app.js                      # Complete client-side application logic
│       ├── config.json                 # Auto-generated contract ABI & test accounts configuration
│       └── lib/
│           └── ethers.umd.min.js       # Bundled Ethers.js library
└── go-ethereum/                        # Custom Go-Ethereum client codebase
    ├── cmd/geth/main.go                # Asset pool CLI flags registered
    ├── cmd/utils/flags.go              # CLI flags definition & parsing logic
    ├── eth/
    │   ├── backend.go                  # Ethereum backend service lifecycle hooks
    │   └── ethconfig/config.go         # Configuration struct defaults for AssetPool
    └── core/
        └── assetpool/                  # ⚠️ MISSING: The Go package implementation!
```

---

## 3. In-Depth Gap Analysis: What Is Finished vs. Unfinished

### ✅ Completed Components

1. **Smart Contract Layer (`contracts/src/ArtworkMarketplace.sol`)**:
   - Implements full sale lifecycle: `Active` $\rightarrow$ `Purchased` $\rightarrow$ `Received` (or `Cancelled`).
   - Implements `postSale` with on-chain cryptographic signature verification (`recoverSigner` supporting standard EIP-191 prefixed hashes and raw Keccak-256 hashes).
   - Supports designated purchaser restrictions.
   - Enforces exact ETH payment transfer directly to the creator upon `purchaseArtwork`.
   - Records `confirmReceipt` by the purchaser.
   - **Tested**: All 4 Foundry test cases in `ArtworkMarketplace.t.sol` pass cleanly (`testPostSaleAndPurchase`, `testCancelSale`, `testDesignatedPurchaserRestriction`, `testInvalidSignatureRejected`).

2. **Frontend Client Application (`artwork-market-platform/web/`)**:
   - Responsive multi-persona interface (Alice the Creator, Bob the Purchaser, Charlie user).
   - **Creator Studio**: Computes client-side `keccak256` hash of raw media files, signs using Ethereum wallet private keys, uploads payload to `/asset/upload`, and calls `postSale` on-chain.
   - **Marketplace Gallery**: Displays listed artworks, verifies creator badges, calculates remaining retention countdowns, and executes `purchaseArtwork`.
   - **Purchaser Vault**: Provides direct download of original media from Geth node (`/asset/{id}`) and invokes `confirmReceipt`.
   - **Node Diagnostics Inspector**: Interacts with `/asset/list`, `/asset/stats`, and `asset_prune` to monitor active node memory/disk usage.

3. **Orchestration Scripts (`artwork-market-platform/scripts/` & `start.sh`)**:
   - `start.sh` is written to build geth, spin up the private node on port 8545, deploy contracts, seed sample SVG artwork, and host the web server on port 3000.

---

### ❌ Unfinished / Missing Components

1. **Missing Package: `go-ethereum/core/assetpool` (Critical Blocker)**:
   - While `go-ethereum/cmd/geth/main.go`, `cmd/utils/flags.go`, `eth/backend.go`, and `eth/ethconfig/config.go` import `"github.com/ethereum/go-ethereum/core/assetpool"` and call its APIs, **the directory `go-ethereum/core/assetpool` does not exist in the repository**.
   - As a result, compiling Geth fails immediately with `package github.com/ethereum/go-ethereum/core/assetpool is not in std`.
   - **Required implementations inside `core/assetpool/`**:
     - `config.go`: Configuration struct containing `Datadir string`, `RetentionWindow time.Duration`, `Enabled bool`, and `DefaultConfig`.
     - `types.go`: Internal representations for `Asset`, `Metadata`, and status enums (`ACTIVE`, `PURCHASED`, `RECEIVED`, `EXPIRED`).
     - `assetpool.go`: Core `AssetPool` manager handling disk storage (saving binary media and JSON metadata under `<datadir>/assetpool/`), memory index, retrieval, and concurrent locking.
     - `crypto.go`: Cryptographic verification engine checking secp256k1 signatures (`crypto.SigToPub`, `crypto.Ecrecover`) matching both EIP-191 prefixed hashes and raw 32-byte Keccak-256 hashes against creator addresses.
     - `http.go`: Direct HTTP streaming handler registered at `/asset/`:
       - `GET /asset/{id}`: Binary media streaming with verification headers (`X-Asset-Creator`, `X-Asset-Verified`, `X-Asset-Status`, `X-Asset-Expires-At`).
       - `POST /asset/upload`: Multipart form and JSON upload processing.
       - `GET /asset/list`: JSON listing of active media assets with seconds remaining.
       - `GET /asset/stats`: Pool telemetry (`activeAssets`, `totalBytes`, `retentionWindow`).
     - `api.go`: JSON-RPC service under the `asset_*` namespace:
       - `asset_uploadAsset(metadata, rawDataHex)`
       - `asset_getAsset(assetId)`
       - `asset_getMetadata(assetId)`
       - `asset_listAssets()`
       - `asset_markPurchased(assetId, purchaser)`
       - `asset_confirmReceived(assetId, purchaser)`
       - `asset_prune()`
       - `asset_getStats()`
     - `prune.go`: Background worker goroutine running periodic sweeps to delete expired assets and acknowledged receipts from disk and memory.

2. **Environment Toolchain (Go Compiler)**:
   - The system currently does not have `go` in the user's `PATH`. A Go toolchain (Go 1.25+ or compatible) must be made available in user space (e.g., in `~/.local/go` or via PATH) to compile the modified Geth binary.

3. **End-to-End Build & Execution**:
   - The Geth binary (`go-ethereum/build/bin/geth`) must be compiled.
   - The testnet must be initialized and verified working with the native Asset Pool HTTP and RPC endpoints.
   - Contracts must be deployed to the live testnet and validated via `seed_sample_art.py` and the web interface.

---

## 4. End-to-End Transaction Flow Diagram

```
+----------------------------------------------------------------------------------------------------+
|                                    1. ARTWORK CREATION & LISTING                                    |
+----------------------------------------------------------------------------------------------------+
 Creator (Alice)
   |
   |-- 1. Selects file (PNG, JPG, SVG)
   |-- 2. Calculates assetId = keccak256(raw_bytes)
   |-- 3. Signs assetId with secp256k1 private key: sig = sign(assetId)
   |
   +---> [POST /asset/upload] --------> Geth Asset Pool Node Subsystem
   |                                      |-- Validates keccak256(file) == assetId
   |                                      |-- Recovers signer from sig == creator
   |                                      |-- Stores file on disk (<datadir>/assetpool/<id>.bin)
   |                                      \-- Sets retention expiry = now + 48 hours
   |
   +---> [postSale(assetId, price, buyer, sig, title)] ----> ArtworkMarketplace.sol
                                                               |-- Verifies recoverSigner(assetId, sig)
                                                               |-- Saves sale record (Status: Active)
                                                               \-- Emits ArtworkPosted

+----------------------------------------------------------------------------------------------------+
|                                      2. PURCHASE & PAYMENT                                         |
+----------------------------------------------------------------------------------------------------+
 Purchaser (Bob)
   |
   |-- 1. Inspects listing in Gallery (verified creator badge, 48h countdown)
   |-- 2. Sends purchaseArtwork(assetId) with msg.value == price
   |
   +---> [purchaseArtwork(assetId)] ----> ArtworkMarketplace.sol
                                            |-- Verifies buyer eligibility (checks designatedPurchaser)
                                            |-- Forwards ETH payment directly to Creator (Alice)
                                            |-- Updates status to Purchased
                                            \-- Emits ArtworkPurchased

+----------------------------------------------------------------------------------------------------+
|                                   3. MEDIA RETRIEVAL & RECEIPT                                      |
+----------------------------------------------------------------------------------------------------+
 Purchaser (Bob)
   |
   |-- 1. Fetches artwork: GET http://localhost:8545/asset/<assetId>
   |-- 2. Verifies local hash & inspects artwork in Purchaser Vault
   |-- 3. Submits on-chain confirmation
   |
   +---> [confirmReceipt(assetId)] ----> ArtworkMarketplace.sol
   |                                       |-- Sets status to Received
   |                                       \-- Emits ArtworkReceived
   |
   +---> [RPC: asset_confirmReceived] -> Geth Asset Pool Node Subsystem
                                           \-- Flags asset as acknowledged for cleanup

+----------------------------------------------------------------------------------------------------+
|                                     4. RETENTION & PRUNING                                         |
+----------------------------------------------------------------------------------------------------+
 Geth Asset Pool Background Worker
   |
   |-- Runs periodic sweep (every 5-10 minutes or upon asset_prune call)
   |-- Evaluates: (now > expiresAt) OR (status == RECEIVED)
   \-- Deletes bin and json files from disk; frees memory
```

---

## 5. Detailed Implementation Plan & Roadmap

### Phase 1: Environment & Toolchain Preparation
- Verify and set up Go 1.25 toolchain in `/home/baller/.local/go` or user environment.
- Ensure `go`, `forge`, `cast`, and `python3` are in PATH and functional.

### Phase 2: Implement `go-ethereum/core/assetpool`
1. **`core/assetpool/config.go`**:
   - `Config` struct (`Datadir`, `RetentionWindow`, `Enabled`).
   - `DefaultConfig` (Default: retention 48h, enabled true, datadir "assetpool").
2. **`core/assetpool/types.go`**:
   - `Asset` struct, `Metadata` struct, JSON serializable types.
3. **`core/assetpool/crypto.go`**:
   - Verification logic verifying secp256k1 signature over Keccak-256 asset ID (handling both `\x19Ethereum Signed Message:\n32` prefix and raw 32-byte hash).
4. **`core/assetpool/assetpool.go`**:
   - Constructor `New(cfg Config, datadir string) (*AssetPool, error)`.
   - Disk read/write methods with directory safety.
   - In-memory registry with `sync.RWMutex`.
   - `Start()` and `Stop()` methods for background pruning loop.
5. **`core/assetpool/http.go`**:
   - HTTP mux handler registered under `/asset/`:
     - `GET /asset/{id}`: Serves binary data with custom HTTP headers (`Content-Type`, `X-Asset-Verified: true`, `X-Asset-Creator: 0x...`).
     - `POST /asset/upload`: Accepts `multipart/form-data` and JSON body.
     - `GET /asset/list`: Returns JSON array of active assets with retention seconds.
     - `GET /asset/stats`: Returns telemetry statistics.
6. **`core/assetpool/api.go`**:
   - Public RPC API struct `AssetAPI` exposing methods for Geth JSON-RPC.
7. **`core/assetpool/prune.go`**:
   - Automatic timer-based cleanup routine and manual `Prune()` method removing stale disk files and updating metrics.

### Phase 3: Build & Geth Integration Verification
- Execute `go build -v -o build/bin/geth ./cmd/geth` inside `go-ethereum/`.
- Verify the build succeeds with no import errors.
- Test Geth flag parsing: `./build/bin/geth --help | grep assetpool`.

### Phase 4: Genesis & Local Testnet Verification
- Test datadir initialization: `geth --datadir ... init genesis.json`.
- Launch dev testnet with `--assetpool.retention 48h --assetpool.datadir ...`.
- Test HTTP endpoint: `curl http://localhost:8545/asset/stats`.
- Test JSON-RPC endpoint: `curl -X POST http://localhost:8545 -d '{"jsonrpc":"2.0","method":"asset_getStats","params":[],"id":1}'`.

### Phase 5: Contract Deployment & Seeding
- Run `deploy_contracts.py` to compile and deploy `ArtworkMarketplace.sol` to the local node.
- Run `seed_sample_art.py` to upload "Aurora Genesis #001" to the Asset Pool and post the sale on-chain.
- Verify contract state via `cast call` to confirm the sale is listed.

### Phase 6: End-to-End Verification via Web Platform
- Start web server at `http://localhost:3000`.
- Verify full workflow:
  1. Alice lists a new custom artwork via Creator Studio.
  2. Bob browses the listing, verifies creator signature badge, and completes purchase with ETH.
  3. Bob downloads the media asset from Geth node and confirms receipt on-chain.
  4. Diagnostics tab verifies asset pool metrics and tests manual pruning.
