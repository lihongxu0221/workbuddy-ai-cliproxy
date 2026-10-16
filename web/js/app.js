const API_BASE = window.location.pathname.replace(/\/ui\/?$/, "");

let currentTab = "accounts";

document.addEventListener("DOMContentLoaded", () => {
  initTabs();
  loadStats();
  loadAccounts();
  loadModels();

  document.getElementById("btn-refresh-all").addEventListener("click", () => {
    loadStats();
    loadAccounts();
    loadModels();
  });

  document.getElementById("btn-open-import").addEventListener("click", () => {
    document.getElementById("modal-import").classList.add("active");
  });

  document.getElementById("btn-close-import").addEventListener("click", () => {
    document.getElementById("modal-import").classList.remove("active");
  });

  document.getElementById("form-import").addEventListener("submit", handleImportSubmit);
});

function initTabs() {
  document.querySelectorAll(".tab-btn").forEach(btn => {
    btn.addEventListener("click", (e) => {
      document.querySelectorAll(".tab-btn").forEach(b => b.classList.remove("active"));
      document.querySelectorAll(".tab-pane").forEach(p => p.style.display = "none");
      btn.classList.add("active");
      currentTab = btn.getAttribute("data-tab");
      document.getElementById(`tab-${currentTab}`).style.display = "block";
    });
  });
}

async function loadStats() {
  try {
    const res = await fetch(`${API_BASE}/stats`);
    const json = await res.json();
    if (json.code === 0 && json.data) {
      document.getElementById("stat-requests").innerText = json.data.today_requests || 0;
      document.getElementById("stat-tokens").innerText = (json.data.total_tokens || 0).toLocaleString();
      document.getElementById("stat-cache").innerText = (json.data.cache_hit_rate || 0).toFixed(1) + "%";
      document.getElementById("stat-credits-spent").innerText = (json.data.total_credit_spent || 0).toFixed(3);
    }
  } catch (e) {
    console.error("load stats failed:", e);
  }
}

async function loadAccounts() {
  const tbody = document.getElementById("accounts-tbody");
  tbody.innerHTML = `<tr><td colspan="7" style="text-align:center;color:var(--text-muted)">Loading accounts...</td></tr>`;

  try {
    const res = await fetch(`${API_BASE}/accounts`);
    const json = await res.json();
    if (json.code === 0 && json.data) {
      if (json.data.length === 0) {
        tbody.innerHTML = `<tr><td colspan="7" style="text-align:center;color:var(--text-muted)">No accounts found in auths/. Click "Import Account" to add.</td></tr>`;
        return;
      }
      tbody.innerHTML = "";
      json.data.forEach(acc => {
        const tr = document.createElement("tr");

        const realmBadge = acc.realm === "global"
          ? `<span class="badge badge-global">Global</span>`
          : `<span class="badge badge-cn">CN</span>`;

        const statusBadge = acc.is_quarantined
          ? `<span class="badge badge-danger">Quarantined (${acc.quarantine_info})</span>`
          : (acc.disabled ? `<span class="badge badge-danger">Disabled</span>` : `<span class="badge badge-ok">Active</span>`);

        const slotBadge = `<span class="badge badge-slot">${acc.in_flight_slots}/2</span>`;

        tr.innerHTML = `
          <td><strong>${acc.nickname || acc.id}</strong><br><small style="color:var(--text-muted)">${acc.user_id}</small></td>
          <td>${realmBadge}</td>
          <td>
            <div class="progress-bar-bg"><div class="progress-bar-fill" style="width: ${Math.min(acc.credits, 100)}%"></div></div>
            <strong>${acc.credits.toFixed(1)}</strong>
          </td>
          <td>${slotBadge}</td>
          <td>${statusBadge}</td>
          <td>
            <div style="display:flex;gap:4px">
              ${acc.realm === "global"
                ? `<button class="btn-secondary btn-sm" onclick="reportBonus('${acc.id}')">Daily Report</button>`
                : `<button class="btn-secondary btn-sm" onclick="doCheckin('${acc.id}')">Check In</button>`}
              <button class="btn-secondary btn-sm" onclick="refreshAccount('${acc.id}')">Refresh</button>
              <button class="btn-secondary btn-sm" style="color:var(--danger)" onclick="deleteAccount('${acc.id}')">Delete</button>
            </div>
          </td>
        `;
        tbody.appendChild(tr);
      });
    }
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="7" style="text-align:center;color:var(--danger)">Error: ${e.message}</td></tr>`;
  }
}

async function loadModels() {
  const tbody = document.getElementById("models-tbody");
  try {
    const res = await fetch(`${API_BASE}/models`);
    const json = await res.json();
    if (json.code === 0 && json.data) {
      tbody.innerHTML = "";
      json.data.forEach(m => {
        const tr = document.createElement("tr");
        tr.innerHTML = `
          <td><code>${m.id}</code></td>
          <td>${m.displayName}</td>
          <td><span class="badge ${m.realm === 'global' ? 'badge-global' : 'badge-cn'}">${m.realm}</span></td>
          <td>${m.vendor}</td>
          <td>x${m.creditMultiplier.toFixed(2)}</td>
          <td><button class="btn-secondary btn-sm" onclick="testModel('${m.id}')">Test Ping</button></td>
        `;
        tbody.appendChild(tr);
      });
    }
  } catch (e) {
    console.error("load models failed:", e);
  }
}

async function reportBonus(id) {
  try {
    const res = await fetch(`${API_BASE}/actions/dailybonus`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id })
    });
    const json = await res.json();
    alert(json.message || (json.code === 0 ? "Reported successfully" : json.error));
    loadAccounts();
  } catch (e) {
    alert("Report failed: " + e.message);
  }
}

async function doCheckin(id) {
  try {
    const res = await fetch(`${API_BASE}/actions/checkin`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id })
    });
    const json = await res.json();
    alert(json.message || "Checkin completed!");
    loadAccounts();
  } catch (e) {
    alert("Checkin failed: " + e.message);
  }
}

async function refreshAccount(id) {
  try {
    const res = await fetch(`${API_BASE}/accounts/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id })
    });
    const json = await res.json();
    loadAccounts();
  } catch (e) {
    alert("Refresh failed: " + e.message);
  }
}

async function deleteAccount(id) {
  if (!confirm(`Are you sure you want to delete ${id}?`)) return;
  try {
    await fetch(`${API_BASE}/accounts/${id}`, { method: "DELETE" });
    loadAccounts();
  } catch (e) {
    alert("Delete failed: " + e.message);
  }
}

async function testModel(model) {
  const btn = event.target;
  const original = btn.innerText;
  btn.innerText = "Testing...";
  try {
    const res = await fetch(`${API_BASE}/models/test`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ model })
    });
    const json = await res.json();
    btn.innerText = original;
    if (json.code === 0) {
      alert(`Test Success!\nModel: ${json.model}\nRealm: ${json.realm}\nCredit Cost: ${json.usage ? json.usage.credit : 'N/A'}`);
    } else {
      alert("Test Failed: " + (json.error || json.message));
    }
  } catch (e) {
    btn.innerText = original;
    alert("Test request error: " + e.message);
  }
}

async function handleImportSubmit(e) {
  e.preventDefault();
  const token = document.getElementById("import-token").value;
  const refreshToken = document.getElementById("import-refresh").value;
  const userId = document.getElementById("import-uid").value;
  const nickname = document.getElementById("import-nickname").value;
  const realm = document.getElementById("import-realm").value;

  try {
    const res = await fetch(`${API_BASE}/accounts/import`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token, refresh_token: refreshToken, user_id: userId, nickname, realm })
    });
    const json = await res.json();
    if (json.code === 0) {
      document.getElementById("modal-import").classList.remove("active");
      loadAccounts();
    } else {
      alert("Import error: " + (json.error || json.message));
    }
  } catch (err) {
    alert("Import failed: " + err.message);
  }
}
