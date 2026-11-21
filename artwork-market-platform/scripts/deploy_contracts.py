#!/usr/bin/env python3
import json
import os
import re
import subprocess
import sys
import time
import urllib.request
import urllib.error

def wait_for_rpc(rpc_url, timeout=30):
    start = time.time()
    payload = json.dumps({"jsonrpc": "2.0", "method": "eth_blockNumber", "params": [], "id": 1}).encode("utf-8")
    req = urllib.request.Request(rpc_url, data=payload, headers={"Content-Type": "application/json"})
    while time.time() - start < timeout:
        try:
            with urllib.request.urlopen(req, timeout=2) as resp:
                if resp.status == 200:
                    data = json.loads(resp.read().decode())
                    if "result" in data:
                        print(f"Geth RPC online at {rpc_url} (block: {data['result']})")
                        return True
        except Exception:
            time.sleep(1)
    return False

def main():
    scripts_dir = os.path.dirname(os.path.abspath(__file__))
    platform_dir = os.path.dirname(scripts_dir)
    contracts_dir = os.path.join(platform_dir, "contracts")
    web_dir = os.path.join(platform_dir, "web")
    os.makedirs(web_dir, exist_ok=True)

    rpc_url = "http://127.0.0.1:8545"
    print(f"Waiting for Geth RPC at {rpc_url}...")
    if not wait_for_rpc(rpc_url):
        print("Error: Geth RPC did not respond within timeout", file=sys.stderr)
        sys.exit(1)

    print("Compiling contracts with forge...")
    subprocess.check_call(["forge", "build"], cwd=contracts_dir)

    deployer_key = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
    print("Deploying ArtworkMarketplace contract via forge create...")
    cmd = [
        "forge", "create",
        "src/ArtworkMarketplace.sol:ArtworkMarketplace",
        "--rpc-url", rpc_url,
        "--private-key", deployer_key,
        "--broadcast"
    ]
    out = subprocess.check_output(cmd, cwd=contracts_dir).decode("utf-8")
    print(out)

    # Extract deployed address
    match = re.search(r"Deployed to:\s*(0x[a-fA-F0-9]{40})", out)
    if not match:
        print("Error: Could not extract deployed address from forge output", file=sys.stderr)
        sys.exit(1)

    contract_address = match.group(1)
    print(f"ArtworkMarketplace successfully deployed at: {contract_address}")

    # Read ABI from out/
    abi_path = os.path.join(contracts_dir, "out", "ArtworkMarketplace.sol", "ArtworkMarketplace.json")
    with open(abi_path, "r") as f:
        artifact = json.load(f)
        abi = artifact.get("abi", [])

    config = {
        "rpcUrl": rpc_url,
        "assetPoolUrl": f"{rpc_url}/asset",
        "chainId": 1337,
        "contractAddress": contract_address,
        "contractAbi": abi,
        "accounts": [
            {
                "name": "Alice (Creator)",
                "address": "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
                "privateKey": "0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d",
                "role": "creator"
            },
            {
                "name": "Bob (Purchaser)",
                "address": "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC",
                "privateKey": "0x5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a",
                "role": "purchaser"
            },
            {
                "name": "Charlie (Purchaser 2)",
                "address": "0x90F79bf6EB2c4f870365E785982E1f101E93b906",
                "privateKey": "0x7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6",
                "role": "purchaser"
            }
        ]
    }

    config_path = os.path.join(web_dir, "config.json")
    with open(config_path, "w") as f:
        json.dump(config, f, indent=2)

    print(f"Web interface configuration written to {config_path}")

if __name__ == "__main__":
    main()
