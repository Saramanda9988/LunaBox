export const ADMIN_HTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="color-scheme" content="light">
  <title>LunaBox 更新观测台</title>
  <style>
    :root {
      --paper: #f2f0e9;
      --paper-deep: #e4e0d4;
      --ink: #132f36;
      --ink-muted: #617078;
      --line: #c7c4b9;
      --signal: #ee6a3b;
      --signal-soft: #ffd8c7;
      --aqua: #8fd9d2;
      --white: #fffef9;
      --shadow: 0 18px 48px rgba(19, 47, 54, .12);
    }
    * { box-sizing: border-box; }
    html { min-width: 320px; background: var(--paper); }
    body {
      margin: 0;
      min-height: 100vh;
      color: var(--ink);
      font-family: "Trebuchet MS", "Microsoft YaHei", sans-serif;
      background:
        linear-gradient(90deg, rgba(19, 47, 54, .035) 1px, transparent 1px) 0 0 / 28px 28px,
        linear-gradient(rgba(19, 47, 54, .035) 1px, transparent 1px) 0 0 / 28px 28px,
        var(--paper);
    }
    button, input { font: inherit; }
    button { cursor: pointer; }
    .shell { width: min(1480px, calc(100% - 40px)); margin: 0 auto; padding: 36px 0 64px; }
    .masthead {
      display: flex;
      align-items: flex-end;
      justify-content: space-between;
      gap: 28px;
      padding: 0 0 24px;
      border-bottom: 2px solid var(--ink);
    }
    .eyebrow, .metric-label, th, .tag, .section-index {
      font-family: Consolas, "Microsoft YaHei", monospace;
      letter-spacing: .08em;
      text-transform: uppercase;
    }
    .eyebrow { margin: 0 0 8px; color: var(--signal); font-size: 12px; font-weight: 700; }
    h1, h2, h3, p { margin-top: 0; }
    h1 { margin-bottom: 0; font: 700 clamp(34px, 5vw, 68px)/.95 Rockwell, "Microsoft YaHei", serif; letter-spacing: -.045em; }
    .status-block { display: flex; align-items: center; gap: 13px; color: var(--ink-muted); font-size: 13px; }
    .status-dot { width: 10px; height: 10px; border-radius: 50%; background: var(--aqua); box-shadow: 0 0 0 5px rgba(143, 217, 210, .3); }
    .actions { display: flex; gap: 8px; margin-top: 14px; justify-content: flex-end; }
    .button {
      border: 1px solid var(--ink);
      background: transparent;
      color: var(--ink);
      padding: 8px 12px;
      font-size: 13px;
      transition: transform .16s ease, background .16s ease, color .16s ease;
    }
    .button:hover { transform: translateY(-2px); background: var(--ink); color: var(--white); }
    .button-primary { background: var(--signal); border-color: var(--signal); color: #26170f; font-weight: 700; }
    .dashboard[hidden], .login-wrap[hidden], .error[hidden], .empty[hidden] { display: none; }
    .login-wrap { display: grid; place-items: center; min-height: 65vh; }
    .login-card {
      width: min(520px, 100%);
      background: var(--white);
      border: 1px solid var(--ink);
      box-shadow: 10px 10px 0 var(--ink);
      padding: clamp(28px, 5vw, 52px);
    }
    .login-card h2 { font: 700 32px/1.05 Rockwell, "Microsoft YaHei", serif; margin-bottom: 12px; }
    .login-card p { color: var(--ink-muted); line-height: 1.65; }
    label { display: block; margin: 28px 0 8px; font-size: 13px; font-weight: 700; }
    .credential-username { position: fixed; left: -10000px; width: 1px; height: 1px; opacity: 0; }
    input {
      width: 100%; border: 1px solid var(--ink); background: var(--paper); color: var(--ink);
      padding: 13px 14px; outline: none;
    }
    input:focus { box-shadow: 0 0 0 3px var(--aqua); }
    .login-card .button { width: 100%; margin-top: 12px; padding: 13px; }
    .error { margin-top: 18px; padding: 12px 14px; border-left: 4px solid var(--signal); background: var(--signal-soft); font-size: 13px; }
    .dashboard { animation: reveal .45s ease both; }
    @keyframes reveal { from { opacity: 0; transform: translateY(12px); } }
    @media (prefers-reduced-motion: reduce) { *, *::before, *::after { animation-duration: .01ms !important; transition-duration: .01ms !important; } }
    .metrics { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); border: 1px solid var(--ink); border-top: 0; background: var(--white); }
    .metric { min-height: 148px; padding: 22px; border-right: 1px solid var(--line); position: relative; overflow: hidden; }
    .metric:last-child { border-right: 0; }
    .metric-accent { background: var(--ink); color: var(--white); }
    .metric-value { display: block; margin-top: 28px; font: 700 clamp(27px, 3.2vw, 48px)/1 Rockwell, "Microsoft YaHei", serif; letter-spacing: -.04em; }
    .metric-label { color: var(--ink-muted); font-size: 11px; }
    .metric-accent .metric-label { color: var(--aqua); }
    .metric-note { margin-top: 8px; color: var(--ink-muted); font-size: 12px; }
    .metric-accent .metric-note { color: #b9c8ca; }
    .content-grid { display: grid; grid-template-columns: minmax(0, 1.35fr) minmax(340px, .65fr); gap: 20px; margin-top: 20px; }
    .panel { border: 1px solid var(--ink); background: rgba(255, 254, 249, .92); box-shadow: var(--shadow); }
    .panel-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; padding: 22px 24px 17px; border-bottom: 1px solid var(--line); }
    .panel-title { margin: 0; font: 700 23px/1.15 Rockwell, "Microsoft YaHei", serif; }
    .panel-subtitle { margin: 5px 0 0; color: var(--ink-muted); font-size: 12px; }
    .section-index { color: var(--signal); font-size: 12px; font-weight: 700; }
    .chart { height: 265px; display: flex; align-items: end; gap: 4px; padding: 26px 24px 22px; }
    .bar-wrap { height: 100%; flex: 1; min-width: 4px; display: flex; flex-direction: column; justify-content: flex-end; align-items: stretch; position: relative; }
    .bar { min-height: 2px; background: var(--aqua); border-top: 2px solid var(--ink); transition: filter .15s ease; }
    .bar-wrap:hover .bar { background: var(--signal); }
    .bar-wrap::after { content: attr(data-label); opacity: 0; pointer-events: none; position: absolute; bottom: calc(var(--height) + 8px); left: 50%; z-index: 2; transform: translateX(-50%); white-space: nowrap; background: var(--ink); color: var(--white); padding: 5px 7px; font: 11px Consolas, monospace; }
    .bar-wrap:hover::after { opacity: 1; }
    .failure-list { padding: 8px 24px 22px; }
    .failure-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 14px; padding: 13px 0; border-bottom: 1px solid var(--line); font-size: 13px; }
    .failure-row:last-child { border-bottom: 0; }
    .failure-code { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .failure-reason { margin-left: 8px; color: var(--ink-muted); font-family: Consolas, monospace; font-size: 12px; }
    .failure-count { font-family: Consolas, monospace; color: var(--signal); font-weight: 700; }
    .wide-panel { margin-top: 20px; }
    .panel-tools { display: flex; align-items: center; gap: 12px; }
    .search { width: min(260px, 40vw); padding: 8px 10px; background: var(--white); }
    .table-scroll { overflow-x: auto; }
    table { width: 100%; border-collapse: collapse; min-width: 850px; }
    th, td { padding: 14px 18px; text-align: left; border-bottom: 1px solid var(--line); }
    th { color: var(--ink-muted); background: var(--paper-deep); font-size: 10px; }
    td { font-size: 13px; }
    tbody tr:hover { background: rgba(143, 217, 210, .16); }
    .version { font: 700 14px Consolas, monospace; }
    .number { font-family: Consolas, monospace; font-variant-numeric: tabular-nums; }
    .tag { display: inline-block; padding: 4px 7px; border: 1px solid var(--ink); background: var(--aqua); font-size: 9px; font-weight: 700; }
    .patch-list { padding: 6px 24px 22px; }
    .patch-card { display: grid; grid-template-columns: minmax(160px, .7fr) 28px minmax(160px, .7fr) 1.5fr; align-items: center; gap: 16px; padding: 18px 0; border-bottom: 1px solid var(--line); }
    .patch-card:last-child { border-bottom: 0; }
    .patch-version { font: 700 16px Consolas, monospace; }
    .patch-arrow { color: var(--signal); font-size: 22px; }
    .patch-meta { color: var(--ink-muted); font-size: 12px; line-height: 1.65; }
    .patch-track { height: 9px; border: 1px solid var(--ink); background: var(--paper-deep); margin-top: 7px; }
    .patch-fill { height: 100%; background: var(--signal); }
    .empty { padding: 35px 24px; color: var(--ink-muted); text-align: center; }
    .warning { padding: 12px 18px; border-top: 1px solid var(--signal); background: var(--signal-soft); color: #69321f; font-size: 12px; }
    @media (max-width: 980px) {
      .metrics { grid-template-columns: repeat(2, 1fr); }
      .metric { border-bottom: 1px solid var(--line); }
      .content-grid { grid-template-columns: 1fr; }
      .patch-card { grid-template-columns: 1fr 24px 1fr; }
      .patch-meta { grid-column: 1 / -1; }
    }
    @media (max-width: 620px) {
      .shell { width: min(100% - 24px, 1480px); padding-top: 20px; }
      .masthead { align-items: flex-start; flex-direction: column; }
      .status-block { width: 100%; justify-content: space-between; }
      .metrics { grid-template-columns: 1fr; }
      .metric { min-height: 116px; border-right: 0; }
      .metric-value { margin-top: 18px; }
      .panel-head { padding: 18px; }
      .panel-tools { align-items: flex-end; flex-direction: column; }
      .search { width: 100%; }
      .patch-list { padding-inline: 18px; }
      .patch-card { grid-template-columns: 1fr; gap: 7px; }
      .patch-arrow { transform: rotate(90deg); }
    }
  </style>
  <script defer src="/admin/app.js"></script>
</head>
<body>
  <main class="shell">
    <header class="masthead">
      <div>
        <p class="eyebrow">Release telemetry / R2 inventory</p>
        <h1>LunaBox 更新观测台</h1>
      </div>
      <div class="status-block">
        <span class="status-dot" aria-hidden="true"></span>
        <span id="statusText">等待身份验证</span>
        <div class="actions">
          <button class="button" id="refreshButton" type="button" hidden>刷新</button>
          <button class="button" id="logoutButton" type="button" hidden>退出</button>
        </div>
      </div>
    </header>

    <section class="login-wrap" id="loginWrap">
      <form class="login-card" id="loginForm">
        <p class="eyebrow">Administrator access</p>
        <h2>读取发布数据</h2>
        <p>输入 Worker 的 <code>ADMIN_TOKEN</code>。令牌保存在当前标签页的会话存储中，关闭标签页后自动清除。</p>
        <input class="credential-username" type="text" name="username" value="administrator" autocomplete="username" tabindex="-1" aria-hidden="true">
        <label for="tokenInput">管理员令牌</label>
        <input id="tokenInput" name="token" type="password" autocomplete="current-password" required autofocus>
        <button class="button button-primary" type="submit">进入观测台</button>
        <div class="error" id="loginError" role="alert" hidden></div>
      </form>
    </section>

    <section class="dashboard" id="dashboard" hidden aria-live="polite">
      <div class="metrics">
        <article class="metric metric-accent"><span class="metric-label">成功更新</span><strong class="metric-value" id="successMetric">—</strong><div class="metric-note">install_success 事件</div></article>
        <article class="metric"><span class="metric-label">设备数</span><strong class="metric-value" id="deviceMetric">—</strong><div class="metric-note">已报告成功的独立设备</div></article>
        <article class="metric"><span class="metric-label">失败次数</span><strong class="metric-value" id="failureMetric">—</strong><div class="metric-note">install_failed 事件</div></article>
        <article class="metric"><span class="metric-label">资源请求</span><strong class="metric-value" id="requestMetric">—</strong><div class="metric-note">R2 更新资源请求</div></article>
        <article class="metric"><span class="metric-label">传输量</span><strong class="metric-value" id="bytesMetric">—</strong><div class="metric-note">按请求区间累计</div></article>
      </div>

      <div class="content-grid">
        <article class="panel">
          <div class="panel-head"><div><span class="section-index">01</span><h2 class="panel-title">近 30 日成功更新</h2><p class="panel-subtitle">按 Worker 接收时间统计</p></div><span class="tag" id="activeVersion">当前发布 —</span></div>
          <div class="chart" id="dailyChart" aria-label="近 30 日成功更新柱状图"></div>
        </article>
        <article class="panel">
          <div class="panel-head"><div><span class="section-index">02</span><h2 class="panel-title">失败原因</h2><p class="panel-subtitle">累计前十项</p></div></div>
          <div class="failure-list" id="failureList"></div>
          <div class="empty" id="failureEmpty" hidden>尚无失败事件</div>
        </article>
      </div>

      <article class="panel wide-panel">
        <div class="panel-head"><div><span class="section-index">03</span><h2 class="panel-title">补丁支持关系</h2><p class="panel-subtitle" id="releaseSummary">正在读取 R2 发布清单</p></div></div>
        <div class="patch-list" id="patchList"></div>
        <div class="empty" id="patchEmpty" hidden>R2 发布清单中尚无补丁资源</div>
        <div class="warning" id="manifestWarning" hidden></div>
      </article>

      <article class="panel wide-panel">
        <div class="panel-head">
          <div><span class="section-index">04</span><h2 class="panel-title">版本更新统计</h2><p class="panel-subtitle">客户端事件与资源请求汇总</p></div>
          <div class="panel-tools"><input class="search" id="versionSearch" type="search" placeholder="筛选版本" aria-label="筛选版本"></div>
        </div>
        <div class="table-scroll">
          <table>
            <thead><tr><th>目标版本</th><th>发现更新</th><th>开始下载</th><th>校验完成</th><th>安装成功</th><th>安装失败</th><th>资源请求</th><th>传输量</th></tr></thead>
            <tbody id="versionRows"></tbody>
          </table>
        </div>
        <div class="empty" id="versionEmpty" hidden>尚无匹配的版本事件</div>
      </article>
    </section>
  </main>
</body>
</html>`;

export const ADMIN_SCRIPT = String.raw`(() => {
  const storageKey = "lunabox-admin-token";
  const state = { data: null, token: sessionStorage.getItem(storageKey) || "" };
  const elements = Object.fromEntries([
    "loginWrap", "loginForm", "tokenInput", "loginError", "dashboard", "statusText",
    "refreshButton", "logoutButton", "successMetric", "deviceMetric", "failureMetric",
    "requestMetric", "bytesMetric", "activeVersion", "dailyChart", "failureList",
    "failureEmpty", "releaseSummary", "patchList", "patchEmpty", "manifestWarning",
    "versionSearch", "versionRows", "versionEmpty",
  ].map(id => [id, document.getElementById(id)]));

  const numberFormat = new Intl.NumberFormat("zh-CN");

  function formatNumber(value) {
    return numberFormat.format(Number(value || 0));
  }

  function formatBytes(value) {
    const bytes = Number(value || 0);
    if (bytes < 1024) return formatNumber(bytes) + " B";
    const units = ["KB", "MB", "GB", "TB"];
    let size = bytes;
    let unit = -1;
    do { size /= 1024; unit += 1; } while (size >= 1024 && unit < units.length - 1);
    return size.toLocaleString("zh-CN", { maximumFractionDigits: size >= 100 ? 0 : 1 }) + " " + units[unit];
  }

  function setAuthenticated(authenticated) {
    elements.loginWrap.hidden = authenticated;
    elements.dashboard.hidden = !authenticated;
    elements.refreshButton.hidden = !authenticated;
    elements.logoutButton.hidden = !authenticated;
  }

  async function loadDashboard() {
    elements.statusText.textContent = "正在读取数据";
    elements.refreshButton.disabled = true;
    try {
      const response = await fetch("/v1/admin/dashboard", {
        headers: { authorization: "Bearer " + state.token },
        cache: "no-store",
      });
      if (response.status === 401) {
        sessionStorage.removeItem(storageKey);
        state.token = "";
        setAuthenticated(false);
        elements.loginError.textContent = "令牌验证失败，请重新输入。";
        elements.loginError.hidden = false;
        elements.statusText.textContent = "等待身份验证";
        return;
      }
      if (!response.ok) throw new Error("服务返回 " + response.status);
      state.data = await response.json();
      render(state.data);
      setAuthenticated(true);
      elements.statusText.textContent = "生成于 " + new Date(state.data.generated_at).toLocaleString("zh-CN");
    } catch (error) {
      const message = error instanceof Error ? error.message : "读取失败";
      elements.statusText.textContent = message;
      if (elements.dashboard.hidden) {
        elements.loginError.textContent = message;
        elements.loginError.hidden = false;
      }
    } finally {
      elements.refreshButton.disabled = false;
    }
  }

  function render(data) {
    elements.successMetric.textContent = formatNumber(data.totals.successful_updates);
    elements.deviceMetric.textContent = formatNumber(data.totals.updated_installations);
    elements.failureMetric.textContent = formatNumber(data.totals.failed_updates);
    elements.requestMetric.textContent = formatNumber(data.totals.download_requests);
    elements.bytesMetric.textContent = formatBytes(data.totals.requested_bytes);
    elements.activeVersion.textContent = "当前发布 " + (data.active_version || "未知");
    renderDaily(data.daily_updates);
    renderFailures(data.failures);
    renderPatches(data.releases, data.invalid_manifests);
    renderVersions(data.versions, elements.versionSearch.value);
  }

  function renderDaily(rows) {
    elements.dailyChart.replaceChildren();
    const byDate = new Map(rows.map(row => [row.date, Number(row.count)]));
    const dates = [];
    const today = new Date();
    today.setHours(12, 0, 0, 0);
    for (let offset = 29; offset >= 0; offset -= 1) {
      const date = new Date(today);
      date.setDate(date.getDate() - offset);
      const key = date.getFullYear() + "-" + String(date.getMonth() + 1).padStart(2, "0") + "-" + String(date.getDate()).padStart(2, "0");
      dates.push({ key, count: byDate.get(key) || 0 });
    }
    const maximum = Math.max(1, ...dates.map(item => item.count));
    for (const item of dates) {
      const wrap = document.createElement("div");
      const height = Math.max(item.count > 0 ? 3 : 1, item.count / maximum * 100);
      wrap.className = "bar-wrap";
      wrap.dataset.label = item.key.slice(5) + " · " + formatNumber(item.count);
      wrap.style.setProperty("--height", height + "%");
      const bar = document.createElement("div");
      bar.className = "bar";
      bar.style.height = height + "%";
      wrap.append(bar);
      elements.dailyChart.append(wrap);
    }
  }

  function renderFailures(rows) {
    elements.failureList.replaceChildren();
    elements.failureEmpty.hidden = rows.length > 0;
    for (const row of rows) {
      const item = document.createElement("div");
      item.className = "failure-row";
      const code = document.createElement("span");
      code.className = "failure-code";
      code.textContent = row.code;
      if (row.reason) {
        const reason = document.createElement("span");
        reason.className = "failure-reason";
        reason.textContent = row.reason;
        code.append(reason);
      }
      const count = document.createElement("span");
      count.className = "failure-count";
      count.textContent = formatNumber(row.count);
      item.append(code, count);
      elements.failureList.append(item);
    }
  }

  function renderPatches(releases, invalidManifests) {
    elements.patchList.replaceChildren();
    const relations = releases.flatMap(release => release.patches.map(patch => ({
      target: release.version,
      uploaded_at: release.uploaded_at,
      source: patch.source_version,
      channels: patch.channels,
    })));
    const patchReleaseCount = new Set(relations.map(item => item.target)).size;
    elements.releaseSummary.textContent = releases.length + " 个发布版本，其中 " + patchReleaseCount + " 个包含补丁资源";
    elements.patchEmpty.hidden = relations.length > 0;
    elements.manifestWarning.hidden = invalidManifests.length === 0;
    elements.manifestWarning.textContent = invalidManifests.length > 0
      ? invalidManifests.length + " 个发布目录缺少有效 manifest.json：" + invalidManifests.join("、")
      : "";
    for (const relation of relations) {
      const card = document.createElement("div");
      card.className = "patch-card";
      const source = document.createElement("div");
      source.innerHTML = '<div class="metric-label">来源版本</div>';
      const sourceVersion = document.createElement("div");
      sourceVersion.className = "patch-version";
      sourceVersion.textContent = relation.source;
      source.append(sourceVersion);
      const arrow = document.createElement("div");
      arrow.className = "patch-arrow";
      arrow.textContent = "→";
      const target = document.createElement("div");
      target.innerHTML = '<div class="metric-label">目标版本</div>';
      const targetVersion = document.createElement("div");
      targetVersion.className = "patch-version";
      targetVersion.textContent = relation.target;
      target.append(targetVersion);
      const averageSaving = relation.channels.length
        ? relation.channels.reduce((total, item) => total + item.saving_percent, 0) / relation.channels.length
        : 0;
      const sizes = relation.channels.map(item => item.patch_size);
      const minSize = Math.min(...sizes);
      const maxSize = Math.max(...sizes);
      const meta = document.createElement("div");
      meta.className = "patch-meta";
      const detail = document.createElement("div");
      detail.textContent = relation.channels.length + " 个渠道 · 补丁 " + formatBytes(minSize) + (minSize === maxSize ? "" : " — " + formatBytes(maxSize)) + " · 平均节省 " + averageSaving.toFixed(1) + "%";
      const track = document.createElement("div");
      track.className = "patch-track";
      const fill = document.createElement("div");
      fill.className = "patch-fill";
      fill.style.width = Math.min(100, Math.max(0, averageSaving)) + "%";
      track.append(fill);
      meta.append(detail, track);
      card.append(source, arrow, target, meta);
      elements.patchList.append(card);
    }
  }

  function renderVersions(rows, query) {
    const normalized = String(query || "").trim().toLowerCase();
    const filtered = rows.filter(row => row.version.toLowerCase().includes(normalized));
    elements.versionRows.replaceChildren();
    elements.versionEmpty.hidden = filtered.length > 0;
    for (const row of filtered) {
      const tr = document.createElement("tr");
      const values = [
        row.version, row.update_available, row.download_started, row.download_verified,
        row.install_success, row.install_failed, row.download_requests, row.requested_bytes,
      ];
      values.forEach((value, index) => {
        const td = document.createElement("td");
        td.className = index === 0 ? "version" : "number";
        td.textContent = index === 0 ? String(value) : index === 7 ? formatBytes(value) : formatNumber(value);
        tr.append(td);
      });
      elements.versionRows.append(tr);
    }
  }

  elements.loginForm.addEventListener("submit", event => {
    event.preventDefault();
    state.token = elements.tokenInput.value.trim();
    if (!state.token) return;
    sessionStorage.setItem(storageKey, state.token);
    elements.loginError.hidden = true;
    loadDashboard();
  });
  elements.refreshButton.addEventListener("click", loadDashboard);
  elements.logoutButton.addEventListener("click", () => {
    sessionStorage.removeItem(storageKey);
    state.token = "";
    state.data = null;
    elements.tokenInput.value = "";
    elements.statusText.textContent = "等待身份验证";
    setAuthenticated(false);
  });
  elements.versionSearch.addEventListener("input", () => {
    if (state.data) renderVersions(state.data.versions, elements.versionSearch.value);
  });

  if (state.token) loadDashboard();
  else setAuthenticated(false);
})();`;

export function adminHTMLResponse(): Response {
  return new Response(ADMIN_HTML, {
    headers: adminHeaders("text/html; charset=utf-8"),
  });
}

export function adminScriptResponse(): Response {
  return new Response(ADMIN_SCRIPT, {
    headers: adminHeaders("text/javascript; charset=utf-8"),
  });
}

function adminHeaders(contentType: string): Headers {
  const headers = new Headers({
    "cache-control": "no-store",
    "content-security-policy": "default-src 'none'; script-src 'self'; style-src 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; base-uri 'none'; form-action 'self'; frame-ancestors 'none'",
    "content-type": contentType,
    "referrer-policy": "no-referrer",
    "x-content-type-options": "nosniff",
    "x-frame-options": "DENY",
  });
  return headers;
}
