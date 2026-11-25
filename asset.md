# Ethereum Native Media Transaction Platform — Project Analysis & Implementation Outline

> **Status: COMPLETED** — All 6 implementation phases, unit tests, smart contract tests, Geth compilation, and end-to-end 2-way media transaction verification are finished and passing.

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

## 2. Directory Structure & Component Map

```
eth-media-transaction/
├── README.md                           # Original project requirements & architecture notes
├── asset.md                            # Project analysis, roadmap, and completion log (this document)
├── .gitignore                          # Scoped to ignore runtime data dirs while tracking source code
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
└── go-ethereum/                        # Custom Go-Ethereum client codebase (v1.17.11-unstable)
    ├── build/bin/geth                  # ✅ Compiled modified Geth binary
    ├── cmd/geth/main.go                # Asset pool CLI flags registered
    ├── cmd/utils/flags.go              # CLI flags definition & parsing logic
    ├── eth/
    │   ├── backend.go                  # Ethereum backend service lifecycle hooks
    │   └── ethconfig/config.go         # Configuration struct defaults for AssetPool
    ├── triedb/                         # ✅ Restored upstream v1.17.11 (78af8c3) triedb packages
    └── core/
        ├── txpool/blobpool/            # ✅ Restored upstream v1.17.11 (78af8c3) blobpool package
        └── assetpool/                  # ✅ IMPLEMENTED: Native Asset Pool package
            ├── config.go               # Config struct & DefaultConfig (48h retention)
            ├── types.go                # AssetStatus, AssetMetadata, AssetListEntry, PoolStats
            ├── crypto.go               # secp256k1 EIP-191 & raw Keccak-256 signature verification
            ├── assetpool.go            # AssetPool manager, disk persistence, lifecycle, pruning engine
            ├── http.go                 # Direct HTTP handler (/asset/upload, /asset/{id}, /asset/list, /asset/stats)
            ├── api.go                  # JSON-RPC service (asset_* namespace)
            └── assetpool_test.go       # Unit tests for lifecycle, signature verification, HTTP, RPC, pruning
```

---

## 3. Component Status Summary (All Completed)

### ✅ Completed Components

1. **Smart Contract Layer (`artwork-market-platform/contracts/src/ArtworkMarketplace.sol`)**:
   - Implements full sale lifecycle: `Active` $\rightarrow$ `Purchased` $\rightarrow$ `Received` (or `Cancelled`).
   - Implements `postSale` with on-chain cryptographic signature verification (`recoverSigner` supporting standard EIP-191 prefixed hashes and raw Keccak-256 hashes).
   - Supports designated purchaser restrictions.
   - Enforces exact ETH payment transfer directly to the creator upon `purchaseArtwork`.
   - Records `confirmReceipt` by the purchaser.
   - **Verified**: All 4 Foundry tests (`testPostSaleAndPurchase`, `testCancelSale`, `testDesignatedPurchaserRestriction`, `testInvalidSignatureRejected`) pass via `forge test`.

2. **Native Geth Asset Pool Subsystem (`go-ethereum/core/assetpool/`)**:
   - **`config.go`**: Exports `Config` (`Datadir`, `RetentionWindow`, `Enabled`) and `DefaultConfig` (`Datadir: "assetpool"`, `RetentionWindow: 48 * time.Hour`, `Enabled: true`).
   - **`types.go`**: Exports `AssetStatus` (`ACTIVE`, `PURCHASED`, `RECEIVED`, `EXPIRED`), `AssetMetadata`, `AssetListEntry`, `PoolStats`, and expiration helpers.
   - **`crypto.go`**: Implements `VerifyCreatorSignature` using `crypto.Ecrecover` / `crypto.Keccak256` supporting both EIP-191 (`\x19Ethereum Signed Message:\n32`) and raw 32-byte Keccak-256 signatures.
   - **`assetpool.go`**: Implements `New(cfg, instanceDir)` (supporting both relative and absolute `--assetpool.datadir` paths), `Start()` / `Stop()` lifecycle hooks, `StoreAsset` (with Keccak-256 hash validation and strict creator signature rejection), `GetAsset`, `GetMetadata`, `ListAssets`, `MarkPurchased`, `ConfirmReceived`, `GetStats`, `Prune()`, disk reload on startup, and 5-minute background `pruneLoop()`.
   - **`http.go`**: Implements `/asset/` HTTP handler supporting `GET`/`HEAD` `/asset/{id}` (with SVG MIME detection and `X-Asset-Creator`, `X-Asset-Verified`, `X-Asset-Status`, `X-Asset-Expires-At` headers), `POST /asset/upload` (multipart & JSON), `GET /asset/list`, `GET /asset/stats`, and CORS preflight headers.
   - **`api.go`**: Implements `AssetAPI` (`NewAssetAPI`) exposing `asset_uploadAsset`, `asset_getAsset`, `asset_getMetadata`, `asset_listAssets`, `asset_markPurchased`, `asset_confirmReceived`, `asset_prune`, and `asset_getStats`.
   - **`assetpool_test.go`**: Unit tests covering valid/spoofed signatures, disk reload, absolute datadir handling, HTTP upload/GET, RPC stats, and pruning. All tests pass (`go test -v ./core/assetpool/...`).

3. **Build, Dependency & Configuration Fixes**:
   - Installed Go `go1.27.4` toolchain to `/home/baller/.local/go` and added `$HOME/.local/go/bin` to `artwork-market-platform/start.sh`.
   - Fixed typo in `go-ethereum/go.mod` (`github.com/dunglas/httpsfv v1.g1.0` $\rightarrow$ `v1.1.0`).
   - Restored `go-ethereum/triedb/` and `go-ethereum/core/txpool/blobpool/` from matching upstream `go-ethereum` commit `78af8c3` (which had been excluded by `.gitignore`).
   - Updated `.gitignore` so runtime directories (`artwork-market-platform/**/assetpool/`, `blobpool/`, `triedb/`) remain ignored without hiding `go-ethereum/` source packages.

4. **Frontend Client Application & Automation (`artwork-market-platform/web/`, `scripts/`, `start.sh`)**:
   - Compiled `go-ethereum/build/bin/geth` cleanly.
   - Verified `start.sh` end-to-end: initializes datadir with `genesis.json`, launches Geth with `--assetpool.retention 48h` and `--http.api eth,net,web3,miner,asset,dev`, deploys `ArtworkMarketplace.sol`, seeds `Aurora Genesis #001`, and serves the Web UI on `http://localhost:3000`.

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
   |                                      |-- Recovers signer from sig == creator (rejects spoofed sigs)
   |                                      |-- Stores file on disk (<datadir>/assetpool/<id>.bin & .json)
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
                                           \-- Flags asset as RECEIVED for cleanup

+----------------------------------------------------------------------------------------------------+
|                                     4. RETENTION & PRUNING                                         |
+----------------------------------------------------------------------------------------------------+
 Geth Asset Pool Background Worker
   |
   |-- Runs periodic sweep (every 5 minutes or upon asset_prune RPC call)
   |-- Evaluates: (now > expiresAt) OR (status == RECEIVED)
   \-- Deletes .bin and .json files from disk; frees memory
```

---

## 5. Implementation Roadmap & Completion Checklist

### Phase 1: Environment & Toolchain Preparation — ✅ Finished
- [x] Installed Go `1.27.4` (`linux/amd64`) into `/home/baller/.local/go`.
- [x] Updated `artwork-market-platform/start.sh` to include `$HOME/.local/go/bin` in `PATH`.
- [x] Verified `go`, `forge`, `cast`, and `python3` are available and operational.
- [x] Fixed `go-ethereum/go.mod` (`httpsfv v1.g1.0` $\rightarrow$ `v1.1.0`) and restored missing upstream `triedb` and `core/txpool/blobpool` packages from commit `78af8c3`.
- [x] Updated `.gitignore` to scope `assetpool/`, `blobpool/`, and `triedb/` ignores to `artwork-market-platform/**/`.

### Phase 2: Implement `go-ethereum/core/assetpool` — ✅ Finished
- [x] `core/assetpool/config.go`: `Config` struct and `DefaultConfig` (48h retention, enabled by default).
- [x] `core/assetpool/types.go`: `AssetStatus`, `AssetMetadata`, `AssetListEntry`, and `PoolStats`.
- [x] `core/assetpool/crypto.go`: `VerifyCreatorSignature` supporting EIP-191 and raw 32-byte Keccak-256 signatures.
- [x] `core/assetpool/assetpool.go`: Disk/memory persistence, signature enforcement, relative/absolute datadir support, background 5-minute pruning loop, and manual `Prune()`.
- [x] `core/assetpool/http.go`: `/asset/` HTTP mux supporting `POST /asset/upload`, `GET`/`HEAD` `/asset/{id}`, `GET /asset/list`, `GET /asset/stats`, CORS, and verification headers.
- [x] `core/assetpool/api.go`: `AssetAPI` JSON-RPC service (`asset_uploadAsset`, `asset_getAsset`, `asset_getMetadata`, `asset_listAssets`, `asset_markPurchased`, `asset_confirmReceived`, `asset_prune`, `asset_getStats`).
- [x] `core/assetpool/assetpool_test.go`: Unit tests passing (`TestAssetPoolLifecycleAndPruning`, `TestAssetPoolRawSignatureAndHTTP`).

### Phase 3: Build & Geth Integration Verification — ✅ Finished
- [x] Built `go-ethereum/build/bin/geth` cleanly with zero compilation errors.
- [x] Verified `--assetpool.datadir`, `--assetpool.retention`, and `--assetpool.disable` flags via `geth --help`.

### Phase 4: Genesis & Local Testnet Verification — ✅ Finished
- [x] Verified datadir initialization via `geth --datadir ... init genesis.json`.
- [x] Launched dev testnet with `--assetpool.retention 48h` and `--http.api eth,net,web3,miner,asset,dev`.
- [x] Verified `GET http://127.0.0.1:8545/asset/stats` and JSON-RPC `asset_getStats`.

### Phase 5: Contract Deployment & Seeding — ✅ Finished
- [x] Verified all 4 Foundry unit tests in `ArtworkMarketplace.t.sol` pass.
- [x] Verified `deploy_contracts.py` compiles and deploys `ArtworkMarketplace.sol` and generates `web/config.json`.
- [x] Verified `seed_sample_art.py` uploads `Aurora Genesis #001` (`0x16fc872147f76592dbc92277ea0b5cb608c6a9f79393d022a799ec0891566caa`) to the Geth Asset Pool and posts the sale on-chain.

### Phase 6: End-to-End Verification via Platform — ✅ Finished
- [x] Verified `artwork-market-platform/start.sh` brings up Geth (`:8545`) and the Web UI (`:3000`).
- [x] Verified `GET` and `HEAD` `/asset/{id}` return `200 OK` with `Content-Type: image/svg+xml` and `X-Asset-Verified: true`.
- [x] Verified spoofed creator signature upload is rejected by the node (`HTTP 400`).
- [x] Verified full 2-way transaction: Alice upload + `postSale` $\rightarrow$ Bob `purchaseArtwork` $\rightarrow$ Bob retrieval from `/asset/{id}` $\rightarrow$ `confirmReceipt` + `asset_confirmReceived` $\rightarrow$ `asset_prune` evicts completed asset.
