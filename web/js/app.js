function getApiBase() {
  const p = window.location.pathname;
  if (p.includes('/v0/resource/plugins/workbuddyai') || p.includes('/v0/management/workbuddyai')) {
    return '/v0/management/workbuddyai';
  }
  if (p.includes('/v0/resource/plugins/workbuddy') || p.includes('/v0/management/workbuddy')) {
    return '/v0/management/workbuddy';
  }
  return p.replace(/\/ui\/?$/, '');
}

function getManagementKey() {
  if (typeof window !== 'undefined' && window.__CPA_MANAGEMENT_KEY__) {
    return window.__CPA_MANAGEMENT_KEY__;
  }
  try {
    return window.localStorage?.getItem('cpa_management_key') ||
           window.localStorage?.getItem('managementKey') ||
           window.parent?.localStorage?.getItem('managementKey') ||
           window.localStorage?.getItem('auth') ||
           window.parent?.localStorage?.getItem('auth') ||
           "";
  } catch (e) {
    return window.localStorage?.getItem('cpa_management_key') || "";
  }
}

function getAuthHeaders(extraHeaders = {}) {
  const headers = { "Content-Type": "application/json", "Accept": "application/json", ...extraHeaders };
  const key = getManagementKey();
  if (key) {
    headers["Authorization"] = "Bearer " + key;
    headers["X-Management-Key"] = key;
  }
  return headers;
}

async function apiFetch(url, options = {}) {
  options.headers = getAuthHeaders(options.headers);
  let res = await fetch(url, options);
  if (res.status === 401) {
    const entered = prompt(
      "【CPA 管理鉴权】请输入 CPA 宿主 Management Key (即 config.yaml 中的 remote-management.secret-key 或 plugins.configs.workbuddyai.management_key):",
      getManagementKey()
    );
    if (entered && entered.trim() !== "") {
      const cleanKey = entered.trim();
      window.localStorage.setItem('cpa_management_key', cleanKey);
      try { window.localStorage.setItem('managementKey', cleanKey); } catch (e) {}
      options.headers = getAuthHeaders(options.headers);
      res = await fetch(url, options);
    }
  }
  return res;
}

const API_BASE = getApiBase();
let currentTab = "accounts";
let pollTimer = null;
let activeAuthURL = "";

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

  const btnRefreshModels = document.getElementById("btn-refresh-models");
  if (btnRefreshModels) {
    btnRefreshModels.addEventListener("click", () => {
      loadModels();
    });
  }

  document.getElementById("btn-set-key").addEventListener("click", () => {
    const current = getManagementKey();
    const entered = prompt(
      "配置 CPA 宿主 Management Key (即 CPA config.yaml 中的 remote-management.secret-key)：\n(若未设置请留空)",
      current
    );
    if (entered !== null) {
      window.localStorage.setItem('cpa_management_key', entered.trim());
      try { window.localStorage.setItem('managementKey', entered.trim()); } catch (e) {}
      alert("密钥已更新！");
      loadStats();
      loadAccounts();
    }
  });

  document.getElementById("btn-open-import").addEventListener("click", () => {
    document.getElementById("modal-import").classList.add("active");
  });

  document.getElementById("btn-close-import").addEventListener("click", () => {
    document.getElementById("modal-import").classList.remove("active");
  });

  document.getElementById("btn-open-login").addEventListener("click", () => {
    document.getElementById("modal-login").classList.add("active");
  });

  document.getElementById("btn-close-login").addEventListener("click", () => {
    document.getElementById("modal-login").classList.remove("active");
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  });

  document.getElementById("btn-start-login").addEventListener("click", handleStartLogin);
  document.getElementById("btn-copy-auth-link").addEventListener("click", handleCopyLink);
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
    const res = await apiFetch(`${API_BASE}/stats`);
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
  tbody.innerHTML = `<tr><td colspan="6" style="text-align:center;color:var(--text-muted)">Loading accounts...</td></tr>`;

  try {
    const res = await apiFetch(`${API_BASE}/accounts`);
    const json = await res.json();
    if (json.code === 0 && json.data) {
      if (json.data.length === 0) {
        tbody.innerHTML = `<tr><td colspan="6" style="text-align:center;color:var(--text-muted)">No accounts found in auths/. Click "扫码 / OAuth 登录" to add.</td></tr>`;
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
    tbody.innerHTML = `<tr><td colspan="6" style="text-align:center;color:var(--danger)">Error: ${e.message}</td></tr>`;
  }
}

async function loadModels() {
  const tbody = document.getElementById("models-tbody");
  tbody.innerHTML = `<tr><td colspan="7" style="text-align:center;color:var(--text-muted)">正在遍历当前账号并探测可用模型...</td></tr>`;

  try {
    const res = await apiFetch(`${API_BASE}/models`);
    const json = await res.json();
    if (json.code === 0 && json.data) {
      if (json.data.length === 0) {
        tbody.innerHTML = `<tr><td colspan="7" style="text-align:center;color:var(--text-muted)">当前账号池暂无可用的模型。请先在「账号池管理」中添加或扫码登录账号。</td></tr>`;
        return;
      }
      tbody.innerHTML = "";
      json.data.forEach(m => {
        const tr = document.createElement("tr");
        const accs = m.availableAccounts && m.availableAccounts.length > 0
          ? m.availableAccounts.map(a => `<span class="badge badge-slot">${a}</span>`).join(" ")
          : `<span class="badge badge-slot">${m.realm}</span>`;

        tr.innerHTML = `
          <td><code>${m.id}</code></td>
          <td>${m.displayName}</td>
          <td><span class="badge ${m.realm === 'global' ? 'badge-global' : 'badge-cn'}">${m.realm}</span></td>
          <td>${accs}</td>
          <td>${m.vendor}</td>
          <td>x${(m.creditMultiplier || 1.0).toFixed(2)}</td>
          <td><button class="btn-secondary btn-sm" onclick="testModel('${m.id}')">Test Ping</button></td>
        `;
        tbody.appendChild(tr);
      });
    }
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="7" style="text-align:center;color:var(--danger)">Error: ${e.message}</td></tr>`;
  }
}

async function reportBonus(id) {
  try {
    const res = await apiFetch(`${API_BASE}/actions/dailybonus`, {
      method: "POST",
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
    const res = await apiFetch(`${API_BASE}/actions/checkin`, {
      method: "POST",
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
    const res = await apiFetch(`${API_BASE}/accounts/refresh`, {
      method: "POST",
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
    await apiFetch(`${API_BASE}/accounts/${encodeURIComponent(id)}`, { method: "DELETE" });
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
    const res = await apiFetch(`${API_BASE}/models/test`, {
      method: "POST",
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
    const res = await apiFetch(`${API_BASE}/accounts/import`, {
      method: "POST",
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

async function handleStartLogin() {
  const realm = document.getElementById("login-realm").value;
  const btn = document.getElementById("btn-start-login");
  btn.disabled = true;
  btn.innerText = "正在获取链接...";

  try {
    const res = await apiFetch(`${API_BASE}/actions/login-start`, {
      method: "POST",
      body: JSON.stringify({ realm })
    });
    const json = await res.json();
    btn.disabled = false;
    btn.innerText = "⚡ 获取授权链接";

    if (json.code === 0 && json.data) {
      const { state, authUrl } = json.data;
      activeAuthURL = authUrl;

      document.getElementById("login-active-box").style.display = "block";
      document.getElementById("login-auth-link").href = authUrl;
      document.getElementById("login-status-label").innerText = "等待授权中，请在打开的页面中完成登录...";
      document.getElementById("login-indicator").style.background = "var(--warning)";

      // 自动新窗口打开
      window.open(authUrl, "_blank");

      // 启动轮询
      if (pollTimer) clearInterval(pollTimer);
      pollTimer = setInterval(async () => {
        try {
          const pollRes = await apiFetch(`${API_BASE}/actions/login-poll?state=${encodeURIComponent(state)}`);
          const pollJson = await pollRes.json();
          if (pollJson.status === "success") {
            clearInterval(pollTimer);
            pollTimer = null;
            document.getElementById("login-status-label").innerText = "✅ 登录成功！账号已成功加入账号池。";
            document.getElementById("login-indicator").style.background = "var(--success)";
            setTimeout(() => {
              document.getElementById("modal-login").classList.remove("active");
              document.getElementById("login-active-box").style.display = "none";
              loadAccounts();
            }, 1500);
          } else if (pollJson.status === "error") {
            clearInterval(pollTimer);
            pollTimer = null;
            document.getElementById("login-status-label").innerText = "❌ 登录失败: " + (pollJson.error || pollJson.message);
            document.getElementById("login-indicator").style.background = "var(--danger)";
          }
        } catch (err) {
          console.warn("poll error:", err);
        }
      }, 2000);
    } else {
      alert("获取授权失败: " + (json.error || json.message));
    }
  } catch (err) {
    btn.disabled = false;
    btn.innerText = "⚡ 获取授权链接";
    alert("请求错误: " + err.message);
  }
}

function handleCopyLink() {
  if (!activeAuthURL) return;
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(activeAuthURL).then(() => {
      alert("授权链接已复制到剪贴板！");
    }).catch(() => {
      prompt("请手动复制授权链接：", activeAuthURL);
    });
  } else {
    prompt("请手动复制授权链接：", activeAuthURL);
  }
}
