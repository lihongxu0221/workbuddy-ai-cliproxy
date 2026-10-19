// =============================================================================
// 1. i18n 多语言环境自适应系统 (自动与 CPAMP 及 CPA 宿主同步)
// =============================================================================

const I18N = {
  "zh-CN": {
    title: "WorkBuddy AI 控制台",
    subtitle: "CLIProxyAPI 企业级双区多账号高可用代理插件",
    btn_refresh_all: "🔄 刷新数据",
    btn_open_login: "⚡ 扫码 / OAuth 登录",
    btn_open_import: "➕ 导入 Token",
    btn_set_key: "🔑 鉴权设置",
    lang_btn: "English",
    stat_requests_title: "今日请求总量",
    stat_tokens_title: "总消耗 Tokens",
    stat_cache_title: "Prompt 缓存命中率",
    stat_credits_title: "累计扣除 Credits",
    tab_accounts: "账号池管理",
    tab_models: "模型目录与探测",
    tab_settings: "运行配置",
    th_acc_info: "账号信息 / UID",
    th_realm: "区域分类",
    th_plan: "当前套餐",
    th_credits: "积分余额 (Credits)",
    th_slots: "活跃并发",
    th_status: "健康状态",
    th_actions: "操作",
    models_tip: "已遍历当前账号池，展示各账号实际授权与可见的模型清单：",
    btn_probe_models: "🔄 重新探测可用模型",
    th_model_id: "模型标识 (Model ID)",
    th_model_name: "显示名称",
    th_model_realm: "所属区域",
    th_model_source: "可用账号来源",
    th_model_vendor: "厂商",
    th_model_rate: "费率倍率",
    th_model_actions: "操作",
    settings_title: "核心参数设置",
    settings_desc: "参数已从 config.yaml 加载并生效。",
    cfg_cpa_compat: "CPA 接口兼容：全兼容 CPA v6 / v7 / v8，支持标准 C ABI 与独立 CLI 代理",
    cfg_bonus_global: "国际区每日 30 积分打卡：已启用 (08:00/12:00/16:00/20:00 4-Tick)",
    cfg_checkin_cn: "国内区每日 100 积分签到：已启用 (09:00/21:00 2-Tick)",
    cfg_max_slots: "单账号并发准入控制：最大 2 槽位 (防封控)",
    cfg_affinity: "软会话粘性 (Affinity)：已开启 (持续保护 Prompt Cache)",
    cfg_prefix: "路由前缀：国内区 cn/，国际区 cb/、en/、wb/",
    modal_login_title: "WorkBuddy 授权登录",
    modal_login_select_label: "选择登录区域与服务",
    opt_realm_global: "国际版 WorkBuddy AI (workbuddy.ai / codebuddy.ai / Google / GitHub / 邮箱)",
    opt_realm_cn: "国内版 WorkBuddy (copilot.tencent.com / workbuddy.cn / 微信 / 手机扫码)",
    btn_fetch_auth_link: "⚡ 获取授权链接",
    login_active_tip: "授权链接已生成，请在新标签页中完成认证：",
    btn_open_login_tab: "🌐 打开登录页",
    btn_copy_auth_link: "复制链接",
    login_waiting_label: "正在等待扫码 / 授权确认中...",
    modal_import_title: "导入 WorkBuddy 账号凭证",
    import_label_realm: "所属区域 (Realm)",
    import_label_token: "Access Token (Bearer JWT) *",
    import_label_refresh: "Refresh Token (可选)",
    import_label_uid: "用户 User ID (UID)",
    import_label_nickname: "显示别名 (Nickname)",
    btn_save_import: "保存凭证",
    btn_daily_report: "Daily Report",
    btn_checkin: "Check In",
    btn_refresh: "Refresh",
    btn_delete: "Delete",
    btn_test: "Test Ping",
    realm_global_title: "国际版 (Global)",
    realm_cn_title: "国内版 (China)",
    scheduler_status_title: "后台定时调度器: 运行中 · 整点排程 (09:00/21:00 每日签到 · 22:00 Token 保活)",
    models_catalog_title: "聚合模型目录",
    alert_copy_ok: "已复制模型 ID: ",
    msg_loading_accounts: "正在加载账号池...",
    msg_no_accounts: "当前未发现账号凭证，请点击「扫码 / OAuth 登录」或「导入 Token」添加。",
    msg_probing_models: "正在遍历当前账号并探测可用模型...",
    msg_no_models: "当前账号池暂无可用的模型。请先在「账号池管理」中添加或扫码登录账号。",
    tip_claim_hint: "可点击右侧 Check In 领取今日 100 积分",
    alert_refresh_ok: "刷新成功！当前可用总积分: ",
    alert_checkin_ok: "签到成功！已领取每日积分奖励: "
  },
  "en": {
    title: "WorkBuddy AI Console",
    subtitle: "Enterprise Dual-Realm Multi-Account High-Availability CPA Plugin",
    btn_refresh_all: "🔄 Refresh Data",
    btn_open_login: "⚡ Scan / OAuth Login",
    btn_open_import: "➕ Import Token",
    btn_set_key: "🔑 Auth Settings",
    lang_btn: "简体中文",
    stat_requests_title: "TODAY'S REQUESTS",
    stat_tokens_title: "TOTAL TOKENS",
    stat_cache_title: "PROMPT CACHE HIT RATE",
    stat_credits_title: "TOTAL CREDITS SPENT",
    tab_accounts: "Account Pool",
    tab_models: "Models & Probing",
    tab_settings: "Settings",
    th_acc_info: "Account / UID",
    th_realm: "Realm",
    th_plan: "Plan",
    th_credits: "Credit Balance",
    th_slots: "Active Slots",
    th_status: "Status",
    th_actions: "Actions",
    models_tip: "Active account pool models dynamically probed and authorized:",
    btn_probe_models: "🔄 Re-probe Usable Models",
    th_model_id: "Model ID",
    th_model_name: "Display Name",
    th_model_realm: "Realm",
    th_model_source: "Account Source",
    th_model_vendor: "Vendor",
    th_model_rate: "Multiplier",
    th_model_actions: "Actions",
    settings_title: "Core Configuration",
    settings_desc: "Configuration loaded and active from config.yaml.",
    cfg_cpa_compat: "CPA Compatibility: Full CPA v6/v7/v8 support with C ABI & Standalone CLI",
    cfg_bonus_global: "Global Daily 30-Credit Report: Enabled (08:00/12:00/16:00/20:00 4-Tick)",
    cfg_checkin_cn: "CN Daily 100-Credit Check-in: Enabled (09:00/21:00 2-Tick)",
    cfg_max_slots: "Admission Gate: Max 2 slots/account (Anti-Abuse Protection)",
    cfg_affinity: "Soft Session Affinity: Enabled (Prompt Cache Preserved)",
    cfg_prefix: "Routing Prefixes: CN realm cn/, Global realm cb/, en/, wb/",
    modal_login_title: "WorkBuddy Authorization Login",
    modal_login_select_label: "Select Login Realm & Service",
    opt_realm_global: "WorkBuddy AI (International / workbuddy.ai / codebuddy.ai / Google / GitHub / Email)",
    opt_realm_cn: "WorkBuddy (Domestic CN / copilot.tencent.com / workbuddy.cn / WeChat / Phone)",
    btn_fetch_auth_link: "⚡ Get Auth Link",
    login_active_tip: "Authorization link generated. Please authenticate in the new tab:",
    btn_open_login_tab: "🌐 Open Login Tab",
    btn_copy_auth_link: "Copy Link",
    login_waiting_label: "Awaiting scan / authorization confirmation...",
    modal_import_title: "Import WorkBuddy Credentials",
    import_label_realm: "Realm",
    import_label_token: "Access Token (Bearer JWT) *",
    import_label_refresh: "Refresh Token (Optional)",
    import_label_uid: "User ID (UID)",
    import_label_nickname: "Display Nickname",
    btn_save_import: "Save Credentials",
    btn_daily_report: "Daily Report",
    btn_checkin: "Check In",
    btn_refresh: "Refresh",
    btn_delete: "Delete",
    btn_test: "Test Ping",
    realm_global_title: "Global Realm",
    realm_cn_title: "China Realm",
    scheduler_status_title: "Scheduler: Running · Scheduled Tasks (09:00/21:00 Daily Claim · 22:00 Token Keepalive)",
    models_catalog_title: "Aggregated Models",
    alert_copy_ok: "Copied model ID: ",
    msg_loading_accounts: "Loading account pool...",
    msg_no_accounts: "No accounts found. Click 'Scan / OAuth Login' or 'Import Token' to add.",
    msg_probing_models: "Iterating through active accounts and probing usable models...",
    msg_no_models: "No usable models found for current account pool. Please add an account first.",
    tip_claim_hint: "Click Check In to claim today's 100 credits",
    alert_refresh_ok: "Refresh successful! Total available credits: ",
    alert_checkin_ok: "Check-in successful! Awarded credits: "
  }
};

let currentLang = "zh-CN";

function detectLanguage() {
  // 1. 尝试从父窗口 (CPAMP) 的 html lang 属性或 storage 中嗅探
  try {
    const parentLang = window.parent?.document?.documentElement?.lang;
    if (parentLang) {
      if (parentLang.toLowerCase().startsWith("en")) return "en";
      if (parentLang.toLowerCase().startsWith("zh")) return "zh-CN";
    }

    const parentStored = window.parent?.localStorage?.getItem("cli-proxy-language");
    if (parentStored) {
      if (parentStored.includes("en")) return "en";
      if (parentStored.includes("zh")) return "zh-CN";
    }
  } catch (e) {}

  // 2. 检查本地存储
  try {
    const localStored = localStorage.getItem("cli-proxy-language");
    if (localStored) {
      if (localStored.includes("en")) return "en";
      if (localStored.includes("zh")) return "zh-CN";
    }
  } catch (e) {}

  // 3. 检查当前页面属性或浏览器默认
  const navLang = navigator.language || "zh-CN";
  return navLang.toLowerCase().startsWith("zh") ? "zh-CN" : "en";
}

function t(key, fallback = "") {
  const dict = I18N[currentLang] || I18N["zh-CN"];
  return dict[key] || fallback || key;
}

function applyLanguage(lang) {
  if (lang !== "zh-CN" && lang !== "en") {
    lang = "en";
  }
  currentLang = lang;
  document.documentElement.lang = lang;

  const dict = I18N[lang] || I18N["zh-CN"];

  // 批量替换 DOM 中的 data-i18n
  document.querySelectorAll("[data-i18n]").forEach(el => {
    const key = el.getAttribute("data-i18n");
    if (dict[key]) {
      el.innerText = dict[key];
    }
  });

  // 更新语言按钮提示
  const langLabel = document.getElementById("lang-label");
  if (langLabel) {
    langLabel.innerText = dict["lang_btn"] || (lang === "zh-CN" ? "English" : "简体中文");
  }

  // 重新渲染列表以更新表格内部文字
  loadAccounts();
  loadModels();
}

// =============================================================================
// 2. API 通信与鉴权管理
// =============================================================================

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
  // 1. 初始化多语言
  currentLang = detectLanguage();
  applyLanguage(currentLang);

  // 监听父级 CPAMP 语言切换
  try {
    window.addEventListener("storage", (e) => {
      if (e.key === "cli-proxy-language") {
        applyLanguage(detectLanguage());
      }
    });

    if (window.parent && window.parent.document && window.parent.document.documentElement) {
      const observer = new MutationObserver(() => {
        const pLang = detectLanguage();
        if (pLang !== currentLang) {
          applyLanguage(pLang);
        }
      });
      observer.observe(window.parent.document.documentElement, { attributes: true, attributeFilter: ["lang"] });
    }
  } catch (e) {}

  document.getElementById("btn-toggle-lang").addEventListener("click", () => {
    const nextLang = currentLang === "zh-CN" ? "en" : "zh-CN";
    try { localStorage.setItem("cli-proxy-language", nextLang); } catch (e) {}
    applyLanguage(nextLang);
  });

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

  const modelSearchInput = document.getElementById("model-search-input");
  if (modelSearchInput) {
    modelSearchInput.addEventListener("input", (e) => {
      modelSearchQuery = e.target.value.trim();
      renderModelsCards();
    });
  }

  const filterPills = document.querySelectorAll("#model-filter-pills .filter-pill");
  filterPills.forEach(pill => {
    pill.addEventListener("click", () => {
      filterPills.forEach(p => p.classList.remove("active"));
      pill.classList.add("active");
      currentModelFilter = pill.getAttribute("data-filter") || "all";
      renderModelsCards();
    });
  });

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
  tbody.innerHTML = `<tr><td colspan="7" style="text-align:center;color:var(--text-muted)">${t("msg_loading_accounts")}</td></tr>`;

  try {
    const res = await apiFetch(`${API_BASE}/accounts`);
    const json = await res.json();
    if (json.code === 0 && json.data) {
      if (json.data.length === 0) {
        tbody.innerHTML = `<tr><td colspan="7" style="text-align:center;color:var(--text-muted)">${t("msg_no_accounts")}</td></tr>`;
        return;
      }
      tbody.innerHTML = "";
      json.data.forEach(acc => {
        const tr = document.createElement("tr");

        const realmBadge = acc.realm === "global"
          ? `<span class="badge badge-global">WorkBuddy AI</span>`
          : `<span class="badge badge-cn">WorkBuddy</span>`;

        const planText = acc.plan || (currentLang === "en" ? "Free Tier" : "免费版");
        const planBadge = `<span class="badge badge-slot" style="background:rgba(64,158,255,0.1);color:var(--primary);border:1px solid rgba(64,158,255,0.25)">${planText}</span>`;

        const statusBadge = acc.is_quarantined
          ? `<span class="badge badge-danger">Quarantined (${acc.quarantine_info})</span>`
          : (acc.disabled ? `<span class="badge badge-danger">Disabled</span>` : `<span class="badge badge-ok">Active</span>`);

        const slotBadge = `<span class="badge badge-slot">${acc.in_flight_slots}/2</span>`;

        // 细分解析积分明细 (赠送积分、签到积分、试用积分等)
        let packageBreakdownHTML = "";
        if (acc.credit_packages && acc.credit_packages.length > 0) {
          const breakdownItems = [];
          acc.credit_packages.forEach(p => {
            if (p.remainingCredits > 0) {
              const pName = p.packageName || "积分包";
              breakdownItems.push(`${pName}: ${p.remainingCredits.toFixed(1)}`);
            }
          });
          if (breakdownItems.length > 0) {
            packageBreakdownHTML = `<div style="font-size:11px;color:var(--text-muted);margin-top:2px">${breakdownItems.join(" | ")}</div>`;
          }
        }

        if (acc.credits === 0 && acc.realm === "cn" && !packageBreakdownHTML) {
          packageBreakdownHTML = `<div style="font-size:11px;color:var(--text-muted);margin-top:2px">${t("tip_claim_hint")}</div>`;
        }

        tr.innerHTML = `
          <td><strong>${acc.nickname || acc.id}</strong><br><small style="color:var(--text-muted)">${acc.user_id}</small></td>
          <td>${realmBadge}</td>
          <td>${planBadge}</td>
          <td>
            <div class="progress-bar-bg"><div class="progress-bar-fill" style="width: ${Math.min(acc.credits, 100)}%"></div></div>
            <strong style="font-size:14px">${acc.credits.toFixed(2)}</strong>
            ${packageBreakdownHTML}
          </td>
          <td>${slotBadge}</td>
          <td>${statusBadge}</td>
          <td>
            <div style="display:flex;gap:4px">
              ${acc.realm === "global"
                ? `<button class="btn-secondary btn-sm" onclick="reportBonus('${acc.id}')">${t("btn_daily_report")}</button>`
                : `<button class="btn-secondary btn-sm" onclick="doCheckin('${acc.id}')">${t("btn_checkin")}</button>`}
              <button class="btn-secondary btn-sm" onclick="refreshAccount('${acc.id}')">${t("btn_refresh")}</button>
              <button class="btn-secondary btn-sm" style="color:var(--danger)" onclick="deleteAccount('${acc.id}')">${t("btn_delete")}</button>
            </div>
          </td>
        `;
        tbody.appendChild(tr);
      });

      // 同步更新顶部双区出口看板数据 (对齐 Qoder2API-Hub 图2)
      let globalCredits = 0, globalCount = 0;
      let cnCredits = 0, cnCount = 0;
      json.data.forEach(acc => {
        if (acc.realm === 'global') {
          globalCredits += (acc.credits || 0);
          globalCount++;
        } else {
          cnCredits += (acc.credits || 0);
          cnCount++;
        }
      });
      const elGC = document.getElementById("stat-global-credits");
      if (elGC) elGC.innerText = globalCredits.toFixed(2);
      const elGA = document.getElementById("stat-global-accs");
      if (elGA) elGA.innerText = globalCount;
      const elCC = document.getElementById("stat-cn-credits");
      if (elCC) elCC.innerText = cnCredits.toFixed(2);
      const elCA = document.getElementById("stat-cn-accs");
      if (elCA) elCA.innerText = cnCount;
    }
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="7" style="text-align:center;color:var(--danger)">Error: ${e.message}</td></tr>`;
  }
}

let allModelsData = [];
let currentModelFilter = "all";
let modelSearchQuery = "";

function fmtK(v) {
  if (!v) return "-";
  const num = Number(v);
  if (num >= 1000000) return (num / 1000000).toFixed(num % 1000000 === 0 ? 0 : 1) + "M";
  if (num >= 1000) return Math.round(num / 1000) + "K";
  return num;
}

function copyText(text) {
  if (navigator.clipboard) {
    navigator.clipboard.writeText(text).then(() => {
      alert((t("alert_copy_ok") || "已复制模型 ID: ") + text);
    }).catch(() => {
      prompt("复制模型 ID:", text);
    });
  } else {
    prompt("复制模型 ID:", text);
  }
}

async function loadModels() {
  const container = document.getElementById("models-container");
  if (!container) return;
  container.innerHTML = `<div style="text-align:center;padding:40px;color:var(--text-muted)">${t("msg_probing_models")}</div>`;

  try {
    const res = await apiFetch(`${API_BASE}/models`);
    const json = await res.json();
    if (json.code === 0 && json.data) {
      allModelsData = json.data;
      renderModelsCards();
    } else {
      container.innerHTML = `<div style="text-align:center;padding:40px;color:var(--danger)">${json.error || json.message}</div>`;
    }
  } catch (e) {
    container.innerHTML = `<div style="text-align:center;padding:40px;color:var(--danger)">Error: ${e.message}</div>`;
  }
}

function renderModelsCards() {
  const container = document.getElementById("models-container");
  if (!container) return;

  let filtered = allModelsData.slice();

  // 1. 类别过滤
  if (currentModelFilter === "free") {
    filtered = filtered.filter(m => m.isFree || m.creditMultiplier === 0);
  } else if (currentModelFilter === "vision") {
    filtered = filtered.filter(m => m.supportsImages || (m.id && (m.id.includes("5v") || m.id.includes("vision"))));
  } else if (currentModelFilter === "tools") {
    filtered = filtered.filter(m => m.supportsTools !== false);
  } else if (currentModelFilter === "reasoning") {
    filtered = filtered.filter(m => m.supportsReasoning || (m.id && (m.id.includes("hy3") || m.id.includes("fable") || m.id.includes("pro"))));
  }

  // 2. 关键词搜索
  if (modelSearchQuery) {
    const q = modelSearchQuery.toLowerCase();
    filtered = filtered.filter(m =>
      (m.id && m.id.toLowerCase().includes(q)) ||
      (m.displayName && m.displayName.toLowerCase().includes(q)) ||
      (m.vendor && m.vendor.toLowerCase().includes(q)) ||
      (m.description && m.description.toLowerCase().includes(q))
    );
  }

  const countElem = document.getElementById("availModelCount");
  if (countElem) {
    countElem.textContent = `(共 ${allModelsData.length} 个 · 当前显示 ${filtered.length} 个 · 1:N 账号供给)`;
  }

  if (filtered.length === 0) {
    container.innerHTML = `<div style="text-align:center;padding:40px;color:var(--text-muted)">${t("msg_no_models")}</div>`;
    return;
  }

  container.innerHTML = "";

  filtered.forEach(m => {
    const row = document.createElement("div");
    row.className = "model-card-row";

    // 1:N 供给账号芯片 (对齐用户指令：模型不再按账号区分，但是可以标记模型由哪些账号提供 1:N)
    const accs = (m.availableAccounts && m.availableAccounts.length > 0)
      ? m.availableAccounts
      : [m.realm === "global" ? "WorkBuddy AI 官方" : "WorkBuddy 官方"];

    const accountChipsHTML = accs.map(accStr => {
      const isGlobal = accStr.includes("WorkBuddy AI") || accStr.includes("global");
      return `<span class="account-chip ${isGlobal ? 'account-chip-global' : 'account-chip-cn'}"><span class="dot">●</span> ${accStr}</span>`;
    }).join(" ");

    // 费率倍率胶囊 (严格还原图1 峰 0.50x → 谷 0.10x 样式)
    let rateHTML = "";
    if (m.isFree || m.creditMultiplier === 0) {
      rateHTML = `<span class="pill-rate pill-free">限时免费 0.00x</span>`;
    } else {
      rateHTML = `<span class="pill-rate">峰 ${(m.creditMultiplier || 1.0).toFixed(2)}x → 谷 ${(m.creditMultiplier * 0.2).toFixed(2)}x</span>`;
    }

    // 能力徽标 (视觉, 工具, 推理, NEW - 严格对齐图1样式)
    const capsHTML = [];
    const hasVision = m.supportsImages || (m.id && (m.id.includes("5v") || m.id.includes("vision")));
    if (hasVision) {
      capsHTML.push(`<span class="pill-cap pill-vision">视觉</span>`);
    }
    if (m.supportsTools !== false) {
      capsHTML.push(`<span class="pill-cap pill-tools">工具</span>`);
    }
    const hasReasoning = m.supportsReasoning || (m.id && (m.id.includes("hy3") || m.id.includes("fable") || m.id.includes("pro")));
    if (hasReasoning) {
      capsHTML.push(`<span class="pill-cap pill-reasoning">推理</span>`);
    }
    if (m.id && (m.id.includes("4.1") || m.id.includes("5.3") || m.id.includes("sol") || m.id.includes("astra"))) {
      capsHTML.push(`<span class="pill-cap pill-new">NEW</span>`);
    }

    // 上下文规格 (如 1M / 200K / 400K - 对齐图1右侧展示)
    const ctxLen = m.contextLength ? fmtK(m.contextLength) : "128K";
    const maxIn = m.maxInputTokens ? fmtK(m.maxInputTokens) : ctxLen;
    const maxOut = m.maxOutputTokens ? fmtK(m.maxOutputTokens) : "8K";
    const specsText = (m.maxInputTokens && m.maxInputTokens !== m.contextLength)
      ? `${ctxLen} / <b>${maxIn}</b> / ${maxOut}`
      : `${ctxLen} / <b>${maxOut}</b>`;

    // 区域与厂商标签
    let realmTag = "";
    if (m.availableRealms && m.availableRealms.length >= 2) {
      realmTag = `<span class="badge" style="background:rgba(139,92,246,0.18);color:#c084fc;border:1px solid rgba(139,92,246,0.4)">双区全支持</span>`;
    } else if (m.realm === "global" || (m.availableRealms && m.availableRealms.includes("global"))) {
      realmTag = `<span class="badge badge-global">仅国际版 (WorkBuddy AI)</span>`;
    } else {
      realmTag = `<span class="badge badge-cn">仅国内版 (WorkBuddy)</span>`;
    }

    const descText = m.description || `${m.vendor || 'WorkBuddy'} 旗舰级大语言模型，具备顶尖智能体执行能力，支持复杂代码逻辑与多轮推理任务。`;

    row.innerHTML = `
      <div class="model-info-col">
        <div class="model-name-line">
          <span class="model-title">${m.displayName || m.id}</span>
          ${realmTag}
          <span class="badge badge-slot" style="font-size:11px">${m.vendor || '官方'}</span>
        </div>
        <div class="model-key-line">
          <span>key: <b>${m.id}</b></span>
          <span class="copy-key-btn" onclick="copyText('${m.id}')" title="点击复制模型 ID">📋 复制</span>
          <span style="color:var(--border-color)">|</span>
          <span style="font-size:11px;color:var(--text-muted)">CPA 路由: <code>cn/${m.id}</code> · <code>cb/${m.id}</code></span>
        </div>
        <div class="model-desc-text">${descText}</div>
        <div class="model-accounts-line">
          <span class="accounts-label">供给账号 (${accs.length}):</span>
          ${accountChipsHTML}
        </div>
      </div>

      <div class="model-pills-col">
        ${rateHTML}
        ${capsHTML.join(" ")}
      </div>

      <div class="model-specs-col">
        <span class="context-spec-text">${specsText}</span>
        <span class="badge-native">原生</span>
        <button class="btn-secondary btn-sm" onclick="testModel('${m.id}')">${t("btn_test")}</button>
      </div>
    `;

    container.appendChild(row);
  });
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
    if (json.code === 0) {
      alert(t("alert_checkin_ok") + (json.reward_credits || 100));
    } else {
      alert("Checkin response: " + (json.error || json.message));
    }
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
    if (json.code === 0 && json.data) {
      alert(t("alert_refresh_ok") + json.data.credits + (json.data.plan ? " [" + json.data.plan + "]" : ""));
    } else {
      alert("Refresh: " + (json.error || json.message));
    }
    loadAccounts();
  } catch (e) {
    alert("Refresh failed: " + e.message);
  }
}

async function deleteAccount(id) {
  if (!confirm(`Are you sure you want to delete ${id}?`)) return;
  try {
    // CPA v8 插件路由只支持精确匹配，故用固定路径 POST /accounts/delete，id 放请求体
    await apiFetch(`${API_BASE}/accounts/delete`, {
      method: "POST",
      body: JSON.stringify({ id })
    });
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
    btn.innerText = t("btn_fetch_auth_link");

    if (json.code === 0 && json.data) {
      const { state, authUrl } = json.data;
      activeAuthURL = authUrl;

      document.getElementById("login-active-box").style.display = "block";
      document.getElementById("login-auth-link").href = authUrl;
      document.getElementById("login-status-label").innerText = t("login_waiting_label");
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
    btn.innerText = t("btn_fetch_auth_link");
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
