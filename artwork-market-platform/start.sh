#!/usr/bin/env bash
set -e

# Export required environment paths
export PATH="$HOME/.foundry/bin:$HOME/.nvm/versions/node/v25.2.0/bin:$PATH"

PLATFORM_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE_ROOT="$(dirname "$PLATFORM_DIR")"
GETH_BIN="$WORKSPACE_ROOT/go-ethereum/build/bin/geth"
DATA_DIR="$PLATFORM_DIR/data"
LOG_DIR="$PLATFORM_DIR/logs"

mkdir -p "$LOG_DIR"

echo "============================================================"
echo "  Ethereum Artwork Marketplace — Native Geth Asset Pool     "
echo "============================================================"

# 1. Build modified geth binary if not built
if [ ! -f "$GETH_BIN" ]; then
    echo "[1/5] Building modified Geth binary with Asset Pool..."
    (cd "$WORKSPACE_ROOT/go-ethereum" && go build -v -o build/bin/geth ./cmd/geth)
else
    echo "[1/5] Modified Geth binary verified at $GETH_BIN"
fi

# 2. Cleanup any stale processes on ports 8545 or 3000
echo "[2/5] Checking ports..."
fuser -k 8545/tcp 2>/dev/null || true
fuser -k 3000/tcp 2>/dev/null || true

# 3. Generate genesis and initialize datadir
if [ ! -f "$PLATFORM_DIR/genesis.json" ]; then
    echo "[3/5] Generating custom genesis with pre-funded accounts..."
    python3 "$PLATFORM_DIR/scripts/generate_genesis.py"
fi

if [ ! -d "$DATA_DIR/geth" ]; then
    echo "[3/5] Initializing Geth local testnet datadir..."
    "$GETH_BIN" --datadir "$DATA_DIR" init "$PLATFORM_DIR/genesis.json"
fi

# 4. Launch modified Geth instance in local testnet mode
echo "[4/5] Starting Modified Geth Node with Native Asset Pool..."
"$GETH_BIN" \
    --datadir "$DATA_DIR" \
    --dev \
    --dev.period 1 \
    --http \
    --http.addr 0.0.0.0 \
    --http.port 8545 \
    --http.corsdomain "*" \
    --http.vhosts "*" \
    --http.api eth,net,web3,miner,asset,dev \
    --assetpool.retention 48h \
    --assetpool.datadir "$DATA_DIR/geth/assetpool" \
    > "$LOG_DIR/geth.log" 2>&1 &
GETH_PID=$!
echo "Geth process started (PID: $GETH_PID), logs: $LOG_DIR/geth.log"

# Function to kill child processes on exit
cleanup() {
    echo ""
    echo "Shutting down testnet services..."
    kill $GETH_PID 2>/dev/null || true
    kill $WEB_PID 2>/dev/null || true
    echo "Services stopped cleanly."
}
trap cleanup EXIT INT TERM

# 5. Deploy smart contracts & seed sample artwork
echo "[5/5] Deploying ArtworkMarketplace smart contracts & seeding..."
python3 "$PLATFORM_DIR/scripts/deploy_contracts.py"
python3 "$PLATFORM_DIR/scripts/seed_sample_art.py" || true

# 6. Launch web interface server
echo "Starting Web Interface on http://localhost:3000..."
python3 -m http.server 3000 --directory "$PLATFORM_DIR/web" > "$LOG_DIR/web.log" 2>&1 &
WEB_PID=$!
echo "Web Server started (PID: $WEB_PID), logs: $LOG_DIR/web.log"

echo ""
echo "============================================================"
echo "🚀 TESTNET & MARKETPLACE READY!"
echo "============================================================"
echo "  Web Interface:   http://localhost:3000"
echo "  Geth JSON-RPC:   http://localhost:8545"
echo "  Geth Asset Pool: http://localhost:8545/asset/"
echo ""
echo "  Pre-funded Test Accounts (10,000 ETH each):"
echo "    Alice (Creator):   0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
echo "    Bob (Purchaser):   0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC"
echo "    Charlie (User 3):  0x90F79bf6EB2c4f870365E785982E1f101E93b906"
echo ""
echo "Press Ctrl+C to terminate the testnet."
echo "============================================================"

# Keep alive to monitor
wait $GETH_PID
