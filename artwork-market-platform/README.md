# Ethereum Artwork Marketplace with Native Geth Asset Pool

This project implements a decentralized 2-way artwork and media transaction system for Ethereum without external platforms like IPFS. Media assets are persisted directly by Ethereum nodes in an ephemeral **Asset Pool** for a configurable 48-hour purchaser time window, backed by cryptographic signature verification and on-chain payment & receipt settlement.

---

## 🏛️ Architecture Overview

```
                      +---------------------------------------+
                      |       Ethereum Blockchain (Geth)      |
                      +---------------------------------------+
                                         |
            +----------------------------+----------------------------+
            |                                                         |
  [Smart Contracts]                                            [Asset Pool (Geth Subsystem)]
  ArtworkMarketplace.sol                                       ephemeral storage (datadir/assetpool)
  - postSale(assetId, price, buyer, sig)                       - Content-addressed via Keccak-256
  - purchaseArtwork(assetId) { payable }                       - Secp256k1 creator signature verification
  - confirmReceipt(assetId)                                    - 48-hour purchaser time window
  - cancelSale(assetId)                                        - Automatic periodic node pruning
            |                                                         |
            |                                                         |
            +----------------------------+----------------------------+
                                         |
                                         v
                         +-------------------------------+
                         |   Interactive Web Interface   |
                         |     http://localhost:3000     |
                         +-------------------------------+
                         |  🎨 Creator Studio            |
                         |  🛒 Marketplace Gallery       |
                         |  📦 Purchaser Vault           |
                         |  ⚙️ Geth Asset Pool Inspector  |
                         +-------------------------------+
```

### 1. Native Geth Client Modifications (`go-ethereum`)
- **Package `core/assetpool`**:
  - Implements ephemeral in-node media storage located at `<datadir>/assetpool/`.
  - **48-Hour Purchaser Window**: Assets are retained with an expiration timestamp (`CreatedAt + RetentionWindow`, default 48h).
  - **Cryptographic Verification**: When an asset is uploaded, the node recalculates `keccak256(content)` and verifies the creator's secp256k1 signature using `crypto.SigToPub` / `crypto.Ecrecover`. Assets with invalid or spoofed signatures are rejected at the node level.
  - **Eviction / Pruning**: Background pruning goroutine periodically sweeps the asset directory, evicting expired media and fully acknowledged receipts to keep node storage costs low.
  - **RPC Namespace (`asset_*`)**:
    - `asset_uploadAsset`: Upload raw media, metadata, and creator signature.
    - `asset_getAsset`: Fetch raw media payload and verification status.
    - `asset_getMetadata`: Fetch metadata and remaining retention time.
    - `asset_listAssets`: List all active unexpired assets in node pool.
    - `asset_markPurchased`: Record purchaser on node.
    - `asset_confirmReceived`: Mark asset received by buyer.
    - `asset_prune`: Manually trigger eviction sweep.
    - `asset_getStats`: Pool statistics (active count, total bytes stored, retention window).
  - **Direct HTTP Streaming (`/asset/`)**:
    - Registered via `stack.RegisterHandler("Asset Pool", "/asset/", ...)` on Geth's canonical HTTP server.
    - `GET /asset/{assetId}`: Direct binary streaming with correct `Content-Type` header and verification headers (`X-Asset-Creator`, `X-Asset-Verified`, `X-Asset-Status`, `X-Asset-Expires-At`).
    - `POST /asset/upload`: Multipart/JSON upload endpoint.
    - `GET /asset/list`: JSON listing of active assets.
    - `GET /asset/stats`: JSON pool diagnostic stats.
  - **CLI Flags**:
    - `--assetpool.retention`: Retention duration (default: `48h0m0s`).
    - `--assetpool.datadir`: Custom folder for media storage.
    - `--assetpool.disable`: Disable the media asset pool.

---

### 2. Smart Contract (`ArtworkMarketplace.sol`)
Located in `contracts/src/ArtworkMarketplace.sol`.
- **Sale Posting**:
  - Creator submits `postSale(assetId, price, designatedPurchaser, signature, title)`.
  - Enforces on-chain signer recovery: `recoverSigner(assetId, signature) == msg.sender`.
  - Supports restricted sales: if `designatedPurchaser != address(0)`, only that address can buy.
- **Purchase with ETH**:
  - Purchaser calls `purchaseArtwork(assetId)` sending exact ETH `msg.value == price`.
  - Verifies purchaser authorization and transfers ETH directly to creator.
  - Emits `ArtworkPurchased` event.
- **Receipt Confirmation**:
  - Purchaser verifies the retrieved media and calls `confirmReceipt(assetId)`.
  - Transitions state to `Received`.
- **Cancellation**:
  - Creator can cancel an unsold posting anytime via `cancelSale(assetId)`.

---

### 3. Bootstrap & Testnet Automation (`start.sh`)
Running `./artwork-market-platform/start.sh`:
1. Compiles the modified Geth binary with Asset Pool (`go-ethereum/build/bin/geth`).
2. Generates a custom genesis (`genesis.json`) with Chain ID `1337` and pre-funded accounts (10,000 ETH each).
3. Launches the modified Geth node with HTTP RPC, mining, and Asset Pool enabled (`--assetpool.retention 48h`).
4. Compiles and deploys `ArtworkMarketplace.sol` via Foundry `forge create`.
5. Seeds initial artwork ("Aurora Genesis #001") into both Geth Asset Pool and the smart contract.
6. Launches the interactive Web Interface at `http://localhost:3000`.

---

## 🚀 Quickstart Guide

### Start the Platform
```bash
./artwork-market-platform/start.sh
```

### Access Points
- **Web Interface**: [http://localhost:3000](http://localhost:3000)
- **Geth JSON-RPC**: `http://localhost:8545`
- **Geth Asset Pool HTTP Endpoint**: `http://localhost:8545/asset/`

### Pre-funded Accounts (10,000 ETH each)
| Persona | Name | Address | Private Key |
|---|---|---|---|
| 🎨 Creator | Alice | `0x70997970C51812dc3A010C7d01b50e0d17dc79C8` | `0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d` |
| 🛒 Purchaser 1 | Bob | `0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC` | `0x5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a` |
| 🛍️ Purchaser 2 | Charlie | `0x90F79bf6EB2c4f870365E785982E1f101E93b906` | `0x7c852118294e51e653712a431582679b204e723b69238f42d4a24564344933a2` |

---

## 🧪 Testing the 2-Way Transaction Workflow

### Step 1: Browse or Switch to Alice (Creator)
1. In the Web UI header, select **Alice (Creator)** from the Actor dropdown.
2. Navigate to **🎨 Creator Studio**.
3. Select an artwork file (PNG, JPG, or SVG).
4. Enter Title, Description, and Price (e.g. `0.1 ETH`).
5. Optional: Enter Bob's address as designated purchaser, or leave blank for a public listing.
6. Click **✍️ Sign & Post Artwork to Blockchain**:
   - The UI computes `keccak256(content)`.
   - Alice cryptographically signs the hash with her private key.
   - The media payload is uploaded directly to Geth's Asset Pool (`POST /asset/upload`).
   - The `postSale` transaction is mined on Ethereum.

### Step 2: Switch to Bob (Purchaser)
1. Select **Bob (Purchaser)** from the Actor dropdown.
2. In the **🖼️ Marketplace Gallery**, view Alice's listed artwork.
3. Notice the green **✓ Verified Creator Sig** badge and the 48h retention countdown timer.
4. Click **🛒 Buy for 0.1 ETH**:
   - The `purchaseArtwork` transaction executes on Ethereum transferring ETH to Alice.
   - Geth records the sale and Bob can immediately retrieve the media directly from `http://localhost:8545/asset/<assetId>`.

### Step 3: Verify & Confirm Receipt in Purchaser Vault
1. Switch to **📦 Purchaser Vault**.
2. View the purchased high-resolution media loaded directly from the Geth node.
3. Click **⬇️ Download Original** to save the file locally.
4. Click **✅ Confirm Received**:
   - Submits `confirmReceipt` on-chain to seal the 2-way transaction.

### Step 4: Inspect Geth Asset Pool Diagnostics
1. Switch to **⚙️ Geth Asset Pool Diagnostics**.
2. Inspect live pool metrics: Active assets, total stored bytes, retention countdowns.
3. Click **🧹 Trigger Asset Pool Prune** to observe the node's eviction engine cleaning completed/expired media.
