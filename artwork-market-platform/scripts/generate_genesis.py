#!/usr/bin/env python3
import json
import os
import subprocess
import sys

def main():
    root_dir = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    workspace_root = os.path.dirname(root_dir)
    geth_bin = os.path.join(workspace_root, "go-ethereum", "build", "bin", "geth")

    if not os.path.isfile(geth_bin):
        print(f"Error: geth binary not found at {geth_bin}", file=sys.stderr)
        sys.exit(1)

    print("Generating base genesis from geth...")
    raw_genesis = subprocess.check_output([geth_bin, "--dev", "dumpgenesis"]).decode("utf-8")
    genesis = json.loads(raw_genesis)

    # Configure prefunded accounts (10,000 ETH each)
    funded_accounts = [
        "f39Fd6e51aad88F6F4ce6aB8827279cffFb92266", # Deployer
        "70997970C51812dc3A010C7d01b50e0d17dc79C8", # Alice (Creator)
        "3C44CdDdB6a900fa2b585dd299e03d12FA4293BC", # Bob (Purchaser)
        "90F79bf6EB2c4f870365E785982E1f101E93b906", # Charlie (Purchaser 2)
    ]

    balance_hex = hex(10000 * 10**18)
    for addr in funded_accounts:
        clean = addr.lower().replace("0x", "")
        if "alloc" not in genesis:
            genesis["alloc"] = {}
        genesis["alloc"][clean] = {"balance": balance_hex}

    out_file = os.path.join(root_dir, "genesis.json")
    with open(out_file, "w") as f:
        json.dump(genesis, f, indent=2)

    print(f"Genesis configuration generated at {out_file}")

if __name__ == "__main__":
    main()
