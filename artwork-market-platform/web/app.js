let config = null;
let provider = null;
let contract = null;
let currentWallet = null;
let currentAccount = null;

// DOM Elements
const nodeStatusEl = document.getElementById("node-status");
const nodeTextEl = document.getElementById("node-text");
const accountSelect = document.getElementById("account-select");
const accountBalanceEl = document.getElementById("account-balance");
const statBlockEl = document.getElementById("stat-block");
const statContractEl = document.getElementById("stat-contract");
const statPoolActiveEl = document.getElementById("stat-pool-active");
const statPoolBytesEl = document.getElementById("stat-pool-bytes");
const artworkGridEl = document.getElementById("artwork-grid");
const vaultGridEl = document.getElementById("vault-grid");
const poolAssetsTableBody = document.getElementById("pool-assets-table-body");

let selectedFileBytes = null;
let selectedFileHash = null;
let selectedFileType = null;

// Init Application
window.addEventListener("DOMContentLoaded", async () => {
  setupTabs();
  setupDropZone();
  setupEventListeners();

  try {
    const res = await fetch("config.json");
    if (!res.ok) throw new Error("Could not load config.json");
    config = await res.json();

    statContractEl.innerText = config.contractAddress;
    statContractEl.title = config.contractAddress;

    provider = new ethers.JsonRpcProvider(config.rpcUrl);

    // Setup accounts dropdown
    accountSelect.innerHTML = "";
    config.accounts.forEach((acc, i) => {
      const opt = document.createElement("option");
      opt.value = i;
      opt.innerText = `${acc.name} (${acc.address.substring(0, 6)}...${acc.address.substring(38)})`;
      accountSelect.appendChild(opt);
    });

    accountSelect.addEventListener("change", handleAccountChange);
    await selectAccount(0);

    // Periodic poll
    startPolling();
    showToast("Connected to Modified Geth local testnet", "success");
  } catch (err) {
    console.error("Initialization error:", err);
    nodeStatusEl.className = "node-status-pill offline";
    nodeStatusEl.querySelector(".status-indicator").className = "status-indicator offline";
    nodeTextEl.innerText = "Geth Offline / Error";
    showToast("Failed to initialize: " + err.message, "error");
  }
});

function setupTabs() {
  const tabBtns = document.querySelectorAll(".tab-btn");
  tabBtns.forEach(btn => {
    btn.addEventListener("click", () => {
      tabBtns.forEach(b => b.classList.remove("active"));
      document.querySelectorAll(".tab-content").forEach(tc => tc.classList.remove("active"));

      btn.classList.add("active");
      const tabId = `${btn.getAttribute("data-tab")}-tab`;
      const tabEl = document.getElementById(tabId);
      if (tabEl) tabEl.classList.add("active");

      if (btn.getAttribute("data-tab") === "marketplace") loadMarketplace();
      if (btn.getAttribute("data-tab") === "vault") loadVault();
      if (btn.getAttribute("data-tab") === "pool-inspector") loadPoolInspector();
    });
  });
}

async function selectAccount(index) {
  const acc = config.accounts[index];
  currentAccount = acc;
  currentWallet = new ethers.Wallet(acc.privateKey, provider);
  contract = new ethers.Contract(config.contractAddress, config.contractAbi, currentWallet);
  await updateBalance();
}

async function handleAccountChange(e) {
  await selectAccount(e.target.value);
  loadMarketplace();
  loadVault();
}

async function updateBalance() {
  if (!currentWallet || !provider) return;
  try {
    const bal = await provider.getBalance(currentWallet.address);
    accountBalanceEl.innerText = `${parseFloat(ethers.formatEther(bal)).toFixed(4)} ETH`;
  } catch (e) {
    console.error("Balance fetch error:", e);
  }
}

function startPolling() {
  setInterval(async () => {
    try {
      const blockNum = await provider.getBlockNumber();
      statBlockEl.innerText = `#${blockNum}`;
      nodeStatusEl.className = "node-status-pill";
      nodeStatusEl.querySelector(".status-indicator").className = "status-indicator";
      nodeTextEl.innerText = `Chain 1337 | Block #${blockNum}`;

      updateBalance();
      updatePoolStats();
    } catch (err) {
      nodeStatusEl.className = "node-status-pill offline";
      nodeStatusEl.querySelector(".status-indicator").className = "status-indicator offline";
      nodeTextEl.innerText = "Geth Disconnected";
    }
  }, 3000);

  loadMarketplace();
  updatePoolStats();
}

async function updatePoolStats() {
  try {
    const res = await fetch(`${config.assetPoolUrl}/stats`);
    if (res.ok) {
      const data = await res.json();
      if (data.stats) {
        statPoolActiveEl.innerText = data.stats.activeAssets;
        const kb = (data.stats.totalBytes / 1024).toFixed(1);
        statPoolBytesEl.innerText = `${kb} KB`;
      }
    }
  } catch (e) {
    console.error("Pool stats error:", e);
  }
}

// --- Marketplace Gallery ---
async function loadMarketplace() {
  if (!contract) return;
  artworkGridEl.innerHTML = `<div style="grid-column: 1/-1; text-align:center; padding:3rem; color:var(--text-secondary)">Loading artwork sales from contract...</div>`;

  try {
    const assetIds = await contract.getAllAssetIds();
    if (assetIds.length === 0) {
      artworkGridEl.innerHTML = `<div style="grid-column: 1/-1; text-align:center; padding:3rem; color:var(--text-secondary)">No artworks posted yet. Head to the Creator Studio tab to post the first artwork!</div>`;
      return;
    }

    artworkGridEl.innerHTML = "";
    for (const id of assetIds) {
      const sale = await contract.getSale(id);
      renderArtworkCard(id, sale, artworkGridEl, false);
    }
  } catch (err) {
    console.error("Error loading marketplace:", err);
    artworkGridEl.innerHTML = `<div style="grid-column: 1/-1; text-align:center; padding:3rem; color:var(--accent-red)">Error loading sales: ${err.message}</div>`;
  }
}

// --- Purchaser Vault ---
async function loadVault() {
  if (!contract || !currentWallet) return;
  vaultGridEl.innerHTML = `<div style="grid-column: 1/-1; text-align:center; padding:3rem; color:var(--text-secondary)">Loading purchased media...</div>`;

  try {
    const assetIds = await contract.getAllAssetIds();
    const myPurchases = [];

    for (const id of assetIds) {
      const sale = await contract.getSale(id);
      if (sale.purchaser.toLowerCase() === currentWallet.address.toLowerCase()) {
        myPurchases.push({ id, sale });
      }
    }

    if (myPurchases.length === 0) {
      vaultGridEl.innerHTML = `<div style="grid-column: 1/-1; text-align:center; padding:3rem; color:var(--text-secondary)">No artworks purchased by ${currentAccount.name} yet.</div>`;
      return;
    }

    vaultGridEl.innerHTML = "";
    for (const { id, sale } of myPurchases) {
      renderArtworkCard(id, sale, vaultGridEl, true);
    }
  } catch (err) {
    console.error("Error loading vault:", err);
    vaultGridEl.innerHTML = `<div style="grid-column: 1/-1; text-align:center; padding:3rem; color:var(--accent-red)">Error loading vault: ${err.message}</div>`;
  }
}

function renderArtworkCard(id, sale, container, isVaultView) {
  const card = document.createElement("div");
  card.className = "artwork-card";

  const priceEth = ethers.formatEther(sale.price);
  const statusNum = Number(sale.status);
  const statusNames = ["Active", "Purchased", "Received", "Cancelled"];
  const statusClasses = ["badge-active", "badge-purchased", "badge-received", "badge-purchased"];
  const statusText = statusNames[statusNum] || "Unknown";
  const statusClass = statusClasses[statusNum] || "badge-active";

  const isCreator = sale.creator.toLowerCase() === currentWallet.address.toLowerCase();
  const isPurchaser = sale.purchaser.toLowerCase() === currentWallet.address.toLowerCase();
  const isDesignated = sale.designatedPurchaser !== ethers.ZeroAddress;
  const isEligibleBuyer = !isDesignated || sale.designatedPurchaser.toLowerCase() === currentWallet.address.toLowerCase();

  // Media URL served directly by Geth Asset Pool HTTP endpoint
  const mediaUrl = `${config.assetPoolUrl}/${id}`;

  card.innerHTML = `
    <div class="artwork-image-container">
      <img src="${mediaUrl}" alt="${sale.title}" onerror="this.src='data:image/svg+xml;utf8,<svg xmlns=\\'http://www.w3.org/2000/svg\\' width=\\'300\\' height=\\'200\\' viewBox=\\'0 0 300 200\\'><rect width=\\'100%\\' height=\\'100%\\' fill=\\'%23162036\\'/><text x=\\'50%\\' y=\\'50%\\' fill=\\'%2364748b\\' dominant-baseline=\\'middle\\' text-anchor=\\'middle\\' font-size=\\'14\\'>Loading / Protected Media</text></svg>'">
      <div class="artwork-badge-container">
        <span class="badge ${statusClass}">${statusText}</span>
        <span class="badge badge-verified" title="Verified by Secp256k1 against Creator">✓ Verified Creator Sig</span>
      </div>
    </div>
    <div class="artwork-details">
      <h3 class="artwork-title">${escapeHtml(sale.title || "Untitled Artwork")}</h3>
      <div class="artwork-meta-row">
        <span class="meta-creator">Creator: <code>${shortAddr(sale.creator)}</code></span>
        <span class="meta-price">${priceEth} ETH</span>
      </div>

      ${isDesignated ? `
        <div style="font-size:0.75rem; color:var(--accent-purple); margin-bottom:0.5rem; background:rgba(139, 92, 246, 0.1); padding:4px 8px; border-radius:4px">
          🔒 Designated for: <code>${shortAddr(sale.designatedPurchaser)}</code>
        </div>
      ` : ''}

      <div class="retention-timer">
        <span>⏱️ Purchaser Time Window:</span>
        <strong id="timer-${id.substring(0, 10)}">48h retention</strong>
      </div>

      <div class="card-actions">
        ${statusNum === 0 && !isCreator ? `
          <button class="btn btn-primary btn-full buy-btn" ${!isEligibleBuyer ? 'disabled title="Only designated purchaser can buy"' : ''}>
            🛒 Buy for ${priceEth} ETH
          </button>
        ` : ''}

        ${(isPurchaser || isCreator) ? `
          <a href="${mediaUrl}" target="_blank" download="${sale.title || 'artwork'}.png" class="btn btn-outline btn-full">
            ⬇️ Download Original
          </a>
        ` : ''}

        ${statusNum === 1 && isPurchaser ? `
          <button class="btn btn-success btn-full confirm-btn" style="margin-top:0.5rem">
            ✅ Confirm Received
          </button>
        ` : ''}

        ${statusNum === 0 && isCreator ? `
          <button class="btn btn-outline btn-full cancel-btn">
            ❌ Cancel Sale
          </button>
        ` : ''}
      </div>
    </div>
  `;

  // Attach button handlers
  const buyBtn = card.querySelector(".buy-btn");
  if (buyBtn) {
    buyBtn.addEventListener("click", () => handlePurchase(id, sale.price, sale.title));
  }

  const confirmBtn = card.querySelector(".confirm-btn");
  if (confirmBtn) {
    confirmBtn.addEventListener("click", () => handleConfirmReceipt(id));
  }

  const cancelBtn = card.querySelector(".cancel-btn");
  if (cancelBtn) {
    cancelBtn.addEventListener("click", () => handleCancelSale(id));
  }

  container.appendChild(card);
}

// --- Action Handlers ---
async function handlePurchase(assetId, price, title) {
  try {
    showToast(`Sending purchase transaction for "${title}"...`, "info");
    const tx = await contract.purchaseArtwork(assetId, { value: price });
    showToast(`Transaction sent! Waiting for block inclusion...`, "info");
    await tx.wait(1);

    showToast(`🎉 Successfully purchased "${title}"! Media retrieved from Geth Asset Pool.`, "success");
    await updateBalance();
    loadMarketplace();
    loadVault();
  } catch (err) {
    console.error("Purchase error:", err);
    showToast("Purchase failed: " + (err.reason || err.message), "error");
  }
}

async function handleConfirmReceipt(assetId) {
  try {
    showToast("Confirming receipt on blockchain...", "info");
    const tx = await contract.confirmReceipt(assetId);
    await tx.wait(1);

    // Also inform node asset pool
    try {
      await fetch(`${config.rpcUrl}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          jsonrpc: "2.0",
          method: "asset_confirmReceived",
          params: [assetId, currentWallet.address],
          id: 1
        })
      });
    } catch (e) {
      console.warn("Could not notify RPC confirmReceived:", e);
    }

    showToast("✅ Delivery receipt confirmed on blockchain!", "success");
    loadMarketplace();
    loadVault();
  } catch (err) {
    console.error("Confirm error:", err);
    showToast("Confirmation failed: " + (err.reason || err.message), "error");
  }
}

async function handleCancelSale(assetId) {
  try {
    showToast("Cancelling sale on blockchain...", "info");
    const tx = await contract.cancelSale(assetId);
    await tx.wait(1);
    showToast("Sale cancelled successfully.", "success");
    loadMarketplace();
  } catch (err) {
    console.error("Cancel error:", err);
    showToast("Cancel failed: " + (err.reason || err.message), "error");
  }
}

// --- Creator Studio Form ---
function setupDropZone() {
  const dropZone = document.getElementById("drop-zone");
  const fileInput = document.getElementById("artwork-file");
  const previewBox = document.getElementById("image-preview");
  const previewImg = document.getElementById("preview-img");
  const fileInfo = document.getElementById("file-info");
  const hashInput = document.getElementById("artwork-hash");

  dropZone.addEventListener("click", () => fileInput.click());

  dropZone.addEventListener("dragover", (e) => {
    e.preventDefault();
    dropZone.classList.add("dragover");
  });

  dropZone.addEventListener("dragleave", () => {
    dropZone.classList.remove("dragover");
  });

  dropZone.addEventListener("drop", (e) => {
    e.preventDefault();
    dropZone.classList.remove("dragover");
    if (e.dataTransfer.files.length > 0) {
      handleFile(e.dataTransfer.files[0]);
    }
  });

  fileInput.addEventListener("change", (e) => {
    if (e.target.files.length > 0) {
      handleFile(e.target.files[0]);
    }
  });

  function handleFile(file) {
    selectedFileType = file.type || "image/png";
    const reader = new FileReader();

    reader.onload = (e) => {
      const arrayBuffer = e.target.result;
      selectedFileBytes = new Uint8Array(arrayBuffer);

      // Compute keccak256 hash of raw file bytes
      selectedFileHash = ethers.keccak256(selectedFileBytes);
      hashInput.value = selectedFileHash;

      // Show preview
      const blob = new Blob([selectedFileBytes], { type: selectedFileType });
      previewImg.src = URL.createObjectURL(blob);
      previewBox.style.display = "flex";
      fileInfo.innerText = `${file.name} — ${(file.size / 1024).toFixed(1)} KB (${selectedFileType})`;
    };

    reader.readAsArrayBuffer(file);
  }
}

function setupEventListeners() {
  const form = document.getElementById("create-artwork-form");
  form.addEventListener("submit", async (e) => {
    e.preventDefault();

    if (!selectedFileBytes || !selectedFileHash) {
      showToast("Please select an artwork file first!", "error");
      return;
    }

    const title = document.getElementById("artwork-title").value.trim();
    const desc = document.getElementById("artwork-desc").value.trim();
    const priceEth = document.getElementById("artwork-price").value;
    const designated = document.getElementById("artwork-designated").value.trim() || ethers.ZeroAddress;

    const submitBtn = document.getElementById("submit-post-btn");
    submitBtn.disabled = true;
    submitBtn.innerText = "Processing & Signing...";

    try {
      const priceWei = ethers.parseEther(priceEth);

      // 1. Creator signs the 32-byte Keccak-256 assetId with Ethereum private key
      showToast("Generating creator's secp256k1 cryptographic signature...", "info");
      const signature = await currentWallet.signMessage(ethers.getBytes(selectedFileHash));

      // 2. Upload media payload directly to Geth's Asset Pool HTTP endpoint
      showToast("Uploading media payload to Geth Asset Pool (48h retention)...", "info");
      const formData = new FormData();
      formData.append("title", title);
      formData.append("description", desc);
      formData.append("creator", currentWallet.address);
      formData.append("signature", signature);
      formData.append("price", priceWei.toString());
      if (designated !== ethers.ZeroAddress) {
        formData.append("designatedPurchaser", designated);
      }
      formData.append("file", new Blob([selectedFileBytes], { type: selectedFileType }), `${title}.png`);

      const uploadResp = await fetch(`${config.assetPoolUrl}/upload`, {
        method: "POST",
        body: formData
      });

      if (!uploadResp.ok) {
        const errJson = await uploadResp.json().catch(() => ({}));
        throw new Error(errJson.error || `Asset Pool upload failed with status ${uploadResp.status}`);
      }

      // 3. Post sale transaction to blockchain contract
      showToast("Broadcasting postSale transaction to Ethereum blockchain...", "info");
      const tx = await contract.postSale(
        selectedFileHash,
        priceWei,
        designated,
        signature,
        title
      );
      showToast("Transaction submitted! Waiting for block confirmation...", "info");
      await tx.wait(1);

      showToast(`🎉 Artwork "${title}" posted successfully to Ethereum & Asset Pool!`, "success");

      // Reset form
      form.reset();
      document.getElementById("image-preview").style.display = "none";
      document.getElementById("file-info").innerText = "";
      selectedFileBytes = null;
      selectedFileHash = null;

      // Switch to Marketplace tab
      document.querySelector('[data-tab="marketplace"]').click();
    } catch (err) {
      console.error("Posting error:", err);
      showToast("Failed to post artwork: " + (err.reason || err.message), "error");
    } finally {
      submitBtn.disabled = false;
      submitBtn.innerText = "✍️ Sign & Post Artwork to Blockchain";
    }
  });

  document.getElementById("refresh-marketplace-btn").addEventListener("click", loadMarketplace);
  document.getElementById("refresh-vault-btn").addEventListener("click", loadVault);
  document.getElementById("refresh-pool-btn").addEventListener("click", loadPoolInspector);

  document.getElementById("prune-pool-btn").addEventListener("click", async () => {
    try {
      showToast("Triggering node Asset Pool prune cycle...", "info");
      const res = await fetch(`${config.rpcUrl}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          jsonrpc: "2.0",
          method: "asset_prune",
          params: [],
          id: 1
        })
      });
      const data = await res.json();
      const count = data.result || 0;
      showToast(`Prune cycle finished! Evicted ${count} expired/completed assets from node.`, "success");
      loadPoolInspector();
      updatePoolStats();
    } catch (e) {
      showToast("Prune failed: " + e.message, "error");
    }
  });
}

// --- Asset Pool Diagnostics Inspector ---
async function loadPoolInspector() {
  poolAssetsTableBody.innerHTML = `<tr><td colspan="9" style="text-align:center; color:var(--text-secondary)">Loading assets from Geth node...</td></tr>`;

  try {
    const res = await fetch(`${config.assetPoolUrl}/list`);
    if (!res.ok) throw new Error("Failed to query /asset/list");
    const data = await res.json();

    if (!data.assets || data.assets.length === 0) {
      poolAssetsTableBody.innerHTML = `<tr><td colspan="9" style="text-align:center; color:var(--text-secondary)">No assets currently in node's Asset Pool.</td></tr>`;
      return;
    }

    poolAssetsTableBody.innerHTML = "";
    data.assets.forEach(asset => {
      const row = document.createElement("tr");
      const hoursRemaining = (asset.secondsRemaining / 3600).toFixed(1);
      const kb = (asset.size / 1024).toFixed(1);

      row.innerHTML = `
        <td><code style="font-size:0.75rem">${asset.id.substring(0, 10)}...${asset.id.substring(58)}</code></td>
        <td><strong>${escapeHtml(asset.title || "Untitled")}</strong></td>
        <td><span class="badge" style="background:#1e293b; color:#94a3b8">${asset.contentType || "image/png"}</span></td>
        <td>${kb} KB</td>
        <td><code>${shortAddr(asset.creator)}</code></td>
        <td><span class="badge ${asset.status === 'ACTIVE' ? 'badge-active' : 'badge-purchased'}">${asset.status}</span></td>
        <td><strong style="color:var(--accent-amber)">${hoursRemaining} hrs</strong></td>
        <td><span class="badge badge-verified">✓ ${asset.verified ? 'Verified' : 'Unverified'}</span></td>
        <td><a href="${config.assetPoolUrl}/${asset.id}" target="_blank" style="color:var(--accent-blue)">GET /asset/${asset.id.substring(0, 8)}...</a></td>
      `;
      poolAssetsTableBody.appendChild(row);
    });
  } catch (err) {
    console.error("Pool inspector error:", err);
    poolAssetsTableBody.innerHTML = `<tr><td colspan="9" style="text-align:center; color:var(--accent-red)">Error: ${err.message}</td></tr>`;
  }
}

// --- Utility Functions ---
function shortAddr(addr) {
  if (!addr || addr === ethers.ZeroAddress) return "None (Public)";
  return `${addr.substring(0, 6)}...${addr.substring(38)}`;
}

function escapeHtml(str) {
  return str.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

function showToast(message, type = "info") {
  const container = document.getElementById("toast-container");
  const toast = document.createElement("div");
  toast.className = `toast ${type}`;
  toast.innerText = message;
  container.appendChild(toast);

  setTimeout(() => {
    toast.style.opacity = "0";
    toast.style.transform = "translateX(100%)";
    setTimeout(() => toast.remove(), 300);
  }, 4000);
}
