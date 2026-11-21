#!/usr/bin/env python3
import json
import os
import subprocess
import sys
import urllib.request
import urllib.error

def main():
    scripts_dir = os.path.dirname(os.path.abspath(__file__))
    platform_dir = os.path.dirname(scripts_dir)
    web_dir = os.path.join(platform_dir, "web")
    config_file = os.path.join(web_dir, "config.json")

    if not os.path.isfile(config_file):
        print("config.json not found, skipping seed.")
        return

    with open(config_file, "r") as f:
        config = json.load(f)

    contract_address = config["contractAddress"]
    rpc_url = config["rpcUrl"]
    asset_pool_url = config["assetPoolUrl"]

    alice = config["accounts"][0] # Alice (Creator)
    bob = config["accounts"][1]   # Bob (Purchaser)

    # 1. Create a beautiful SVG artwork
    sample_svg = """<svg xmlns="http://www.w3.org/2000/svg" width="600" height="400" viewBox="0 0 600 400">
  <defs>
    <linearGradient id="skyGrad" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#0a1128"/>
      <stop offset="50%" stop-color="#1c2541"/>
      <stop offset="100%" stop-color="#3a506b"/>
    </linearGradient>
    <linearGradient id="aurora" x1="0%" y1="0%" x2="100%" y2="0%">
      <stop offset="0%" stop-color="#48cae4" stop-opacity="0.2"/>
      <stop offset="30%" stop-color="#52b788" stop-opacity="0.8"/>
      <stop offset="60%" stop-color="#70e000" stop-opacity="0.9"/>
      <stop offset="100%" stop-color="#38b000" stop-opacity="0.1"/>
    </linearGradient>
    <linearGradient id="mountainGrad" x1="0%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#111827"/>
      <stop offset="100%" stop-color="#030712"/>
    </linearGradient>
  </defs>
  <rect width="600" height="400" fill="url(#skyGrad)"/>
  <!-- Stars -->
  <circle cx="80" cy="50" r="1.5" fill="#fff" opacity="0.8"/>
  <circle cx="210" cy="80" r="1" fill="#fff" opacity="0.6"/>
  <circle cx="340" cy="40" r="1.5" fill="#fff" opacity="0.9"/>
  <circle cx="480" cy="70" r="1.2" fill="#fff" opacity="0.7"/>
  <circle cx="520" cy="110" r="2" fill="#90e0ef" opacity="0.9"/>
  <!-- Aurora Waves -->
  <path d="M 0 160 Q 150 90 300 140 T 600 120 L 600 240 Q 450 200 300 230 T 0 220 Z" fill="url(#aurora)" filter="blur(18px)"/>
  <!-- Mountain Silhouette -->
  <polygon points="0,400 120,250 250,340 380,210 500,320 600,260 600,400" fill="url(#mountainGrad)"/>
  <!-- Text Label -->
  <text x="30" y="375" fill="#f8fafc" font-family="sans-serif" font-size="18" font-weight="bold">Aurora Genesis #001</text>
  <text x="30" y="390" fill="#94a3b8" font-family="sans-serif" font-size="11">Secp256k1 Signed On-Chain Media - Persisted in Geth Asset Pool</text>
</svg>"""

    svg_bytes = sample_svg.encode("utf-8")

    # Compute keccak256 hash using cast or python
    hash_output = subprocess.check_output(["cast", "keccak", sample_svg]).decode("utf-8").strip()
    asset_id = hash_output
    print(f"Sample Artwork Keccak-256 Hash: {asset_id}")

    # Sign with Alice private key using cast wallet sign
    # Note: cast wallet sign signs the message with EIP-191 personal_sign
    sig_output = subprocess.check_output([
        "cast", "wallet", "sign",
        "--private-key", alice["privateKey"],
        asset_id
    ]).decode("utf-8").strip()
    signature = sig_output
    print(f"Alice Creator Signature: {signature}")

    # Upload to Geth Asset Pool via RPC or multipart HTTP
    upload_url = f"{asset_pool_url}/upload"
    boundary = "----WebKitFormBoundarySampleArt12345"
    body = (
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="title"\r\n\r\n'
        f"Aurora Genesis #001\r\n"
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="description"\r\n\r\n'
        f"Generative Aurora over mountain silhouette. Native Geth media asset test.\r\n"
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="creator"\r\n\r\n'
        f"{alice['address']}\r\n"
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="signature"\r\n\r\n'
        f"{signature}\r\n"
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="price"\r\n\r\n'
        f"{int(0.05 * 10**18)}\r\n"
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="file"; filename="aurora_genesis_001.svg"\r\n'
        f"Content-Type: image/svg+xml\r\n\r\n"
        f"{sample_svg}\r\n"
        f"--{boundary}--\r\n"
    ).encode("utf-8")

    req = urllib.request.Request(
        upload_url,
        data=body,
        headers={"Content-Type": f"multipart/form-data; boundary={boundary}"}
    )

    try:
        with urllib.request.urlopen(req) as resp:
            print(f"Uploaded to Geth Asset Pool: status {resp.status}")
    except Exception as e:
        print(f"Warning: upload to asset pool gave: {e}")

    # Post sale to contract via cast send
    price_wei = str(int(0.05 * 10**18))
    cmd = [
        "cast", "send",
        contract_address,
        "postSale(bytes32,uint256,address,bytes,string)",
        asset_id,
        price_wei,
        "0x0000000000000000000000000000000000000000",
        signature,
        "Aurora Genesis #001",
        "--rpc-url", rpc_url,
        "--private-key", alice["privateKey"]
    ]
    subprocess.check_call(cmd)
    print("Sample artwork 'Aurora Genesis #001' posted to blockchain contract!")

if __name__ == "__main__":
    main()
