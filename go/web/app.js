(function () {
  "use strict";
  const $ = (s, r = document) => r.querySelector(s);
  const t = (k, d) => (window.KeyLabI18n ? window.KeyLabI18n.t(k, d) : d);
  const $$ = (s, r = document) => Array.from(r.querySelectorAll(s));
  const esc = s => String(s ?? "").replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));

  let home = { servers: [], packages: [], indexSerial: "" };
  let selected = "";          // Lynis panelinde gösterilen sunucu
  let query = "";
  let view = "dash";

  /* ---------------- API ---------------- */
  async function api(path, body) {
    const r = await fetch(path, { method: body ? "POST" : "GET", headers: { "X-Requested-With": "sshpocket" }, body: body ? JSON.stringify(body) : undefined });
    if (r.status === 401) throw new Error("oturum yok: terminalde yazılan adresi açın");
    if (!r.ok) throw new Error((await r.text()).trim() || r.status);
    return r;
  }
  const json = async (p, b) => (await api(p, b)).json();
  function toast(t) {
    const e = $("#toast"); e.textContent = t; e.style.display = "block";
    clearTimeout(toast.h); toast.h = setTimeout(() => (e.style.display = "none"), 2400);
  }

  /* ---------------- Biçim yardımcıları ---------------- */
  function ago(iso) {
    const t = Date.parse(iso); if (!t) return "";
    const s = Math.max(0, (Date.now() - t) / 1000);
    if (s < 90) return "şimdi";
    if (s < 3600) return Math.round(s / 60) + " dk";
    if (s < 86400) return Math.round(s / 3600) + " sa";
    return Math.round(s / 86400) + " gün";
  }
  const localTime = iso => { const d = new Date(iso); return isNaN(d) ? iso : d.toLocaleString("tr-TR"); };
  const isOk = r => r === "ok";
  const resHtml = a => !a ? '<span class="res none">— kayıt yok</span>'
    : isOk(a.result) ? '<span class="res ok">✔ başarılı</span>' : '<span class="res fail">✖ başarısız</span>';
  const matches = (...parts) => !query || parts.join(" ").toLowerCase().includes(query);

  /* ---------------- Gezinme ---------------- */
  function showView(name) {
    view = name;
    $$(".view").forEach(v => (v.hidden = v.id !== "view-" + name));
    const navName = name === "server" || name === "add" ? "servers" : name;
    $$(".nav-item[data-nav]").forEach(b => b.setAttribute("aria-current", b.dataset.nav === navName ? "page" : "false"));
    if (name !== "server" && location.hash) history.replaceState(null, "", location.pathname);
    $(".aside").style.display = name === "dash" ? "" : "none";
    const app = $(".app");
    app.classList.toggle("v-wide", name !== "dash");
    app.classList.toggle("v-server", name === "server");
    $("#srvtree").hidden = name !== "server";
    if (name === "logs") loadLogs();
    if (name === "lynis") loadLynis();
    if (name === "packages") loadPackages();
    if (name === "add") initAdd();
    if (name === "settings") loadSettings();
  }
  $$(".nav-item[data-nav]").forEach(b => b.addEventListener("click", () => showView(b.dataset.nav)));
  $$("[data-nav-target]").forEach(b => b.addEventListener("click", () => showView(b.dataset.navTarget)));

  /* ---------------- Çizimler ---------------- */
  function serverRow(s) {
    const tag = s.platform ? `<span class="tag">${esc(s.platform)}</span>` : "";
    const last = s.last
      ? `${resHtml(s.last)} <span class="muted mono" title="${esc(localTime(s.last.time))}">${esc(ago(s.last.time))}</span>`
      : '<span class="res none">— kayıt yok</span>';
    const sub = s.last ? `<div class="srv-addr" style="margin-top:2px">${esc(s.last.package)} ${esc(s.last.version)} · ${esc(s.last.action)}</div>` : "";
    const addr = s.valid ? `${s.user}@${s.host}:${s.port}` : "server.env geçersiz";
    return `<li class="server" tabindex="0" data-name="${esc(s.name)}" data-state="${dotState(s)}">
      <span class="status-dot ${dotState(s)}" aria-label="durum: ${stateText(s)}"></span>
      <div class="srv-ident"><div class="srv-name">${esc(s.name)} ${tag} ${s.hasKey ? "" : '<span class="tag">anahtar yok</span>'}</div>
        <div class="srv-addr">${esc(addr)}</div>${sub}</div>
      <div class="srv-last">${last}</div>
      <div class="srv-actions"><button class="btn-icon" data-act="menu" aria-label="Daha fazla eylem">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><circle cx="5" cy="12" r="1.8"/><circle cx="12" cy="12" r="1.8"/><circle cx="19" cy="12" r="1.8"/></svg></button></div>
    </li>`;
  }
  /* İşletim sistemi simgeleri: kendi çizdiğimiz sade işaretler (resmi logo değil), 48x48 karo.
     Resmi logoları kullanmak istersek burada aynı anahtarlarla değiştirilir. */
  const OS_LABEL = { ubuntu: "Ubuntu", debian: "Debian", proxmox: "Proxmox VE", linux: "Linux" };
  const OS_ICON = {
    ubuntu: '<rect width="48" height="48" rx="11" fill="#E95420"/><circle cx="24" cy="24" r="10.5" fill="none" stroke="#fff" stroke-width="2.6"/><circle cx="13.5" cy="24" r="4" fill="#fff"/><circle cx="29.2" cy="14.9" r="4" fill="#fff"/><circle cx="29.2" cy="33.1" r="4" fill="#fff"/><circle cx="13.5" cy="24" r="1.6" fill="#E95420"/><circle cx="29.2" cy="14.9" r="1.6" fill="#E95420"/><circle cx="29.2" cy="33.1" r="1.6" fill="#E95420"/>',
    debian: '<rect width="48" height="48" rx="11" fill="#A80030"/><path d="M29.5 12.5c-9-3.2-18 2.3-17.5 12 .4 7.6 7 12 13.4 10.6 5-1.1 8-5.4 6.6-9.6-1.1-3.3-4.8-4.6-7.6-3.2-2.6 1.3-3 4.8-.8 6.4" fill="none" stroke="#fff" stroke-width="3" stroke-linecap="round"/>',
    proxmox: '<rect width="48" height="48" rx="11" fill="#2b2f36"/><path d="M13 13l22 22M35 13L13 35" stroke="#E57000" stroke-width="5.5" stroke-linecap="round"/><path d="M13 13l22 22M35 13L13 35" stroke="#fff" stroke-opacity=".0" stroke-width="1"/>',
    linux: '<rect width="48" height="48" rx="11" fill="var(--surface-3)"/><rect x="12" y="13" width="24" height="9" rx="2.5" fill="none" stroke="var(--muted)" stroke-width="2.4"/><rect x="12" y="26" width="24" height="9" rx="2.5" fill="none" stroke="var(--muted)" stroke-width="2.4"/><circle cx="17" cy="17.5" r="1.4" fill="var(--muted)"/><circle cx="17" cy="30.5" r="1.4" fill="var(--muted)"/>',
  };
  const osIcon = (os, size = 44) => `<svg class="osico" width="${size}" height="${size}" viewBox="0 0 48 48" role="img" aria-label="${esc(OS_LABEL[os] || "Linux")}">${OS_ICON[os] || OS_ICON.linux}</svg>`;

  let mode = "tiles";   // tiles | list
  try { mode = localStorage.getItem("sshpocket.mode") === "list" ? "list" : "tiles"; } catch (e) { /* tarayıcı depolaması kapalı olabilir */ }

  function tile(s) {
    const st = dotState(s);
    const last = !s.valid ? "⚠ server.env geçersiz (IP, USER, PORT)" : s.last ? `${isOk(s.last.result) ? "✔" : "✖"} ${esc(s.last.package)} · ${esc(s.last.action)} · ${esc(ago(s.last.time))}` : "kayıt yok";
    return `<li class="stile ${st === "off" ? "is-off" : ""}" tabindex="0" data-name="${esc(s.name)}" data-state="${st}" aria-label="${esc(s.name)}, ${stateText(s)}">
      <div class="stile-top">${osIcon(s.os)}<span class="status-dot ${st}" aria-label="${stateText(s)}"></span>
        <button class="btn-icon" data-act="menu" aria-label="Daha fazla eylem"><svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><circle cx="5" cy="12" r="1.8"/><circle cx="12" cy="12" r="1.8"/><circle cx="19" cy="12" r="1.8"/></svg></button></div>
      <div class="stile-name">${esc(s.name)}</div>
      <div class="stile-os">${esc(OS_LABEL[s.os] || "Linux")}${s.platform ? ` · ${esc(s.platform)}` : ""}${s.hasKey ? "" : " · anahtar yok"}</div>
      <div class="stile-last ${!s.valid || (s.last && !isOk(s.last.result)) ? "bad" : ""}">${last}</div></li>`;
  }

  // Durumlar: on (yeşil) · lock (turuncu, knock gerekir) · closed (turuncu, makine yanıt veriyor ama port kapalı) · off (kırmızı, yanıt yok)
  const dotState = s => (!s.valid ? "bad" : !s.checked ? "unk" : s.up ? "on" : s.knock ? "lock" : s.conn === "refused" ? "closed" : "off");
  const stateText = s => (!s.valid ? t("st_invalid", "yapılandırma geçersiz") : !s.checked ? t("st_checking", "denetleniyor") : s.up ? t("st_up", "çevrimiçi") : s.knock ? t("st_knock", "🔒 kapalı — knock gerekir") : s.conn === "refused" ? t("st_closed", "port kapalı (makine yanıt veriyor)") : t("st_down", "bağlantı yok"));
  const emptyServers = '<li class="server" style="cursor:default"><div class="srv-ident"><div class="srv-name">Sunucu bulunamadı</div><div class="srv-addr">USB\'de servers/&lt;ad&gt;/server.env yok ya da arama eşleşmedi</div></div></li>';

  function pkgRow(p) {
    return `<div class="pkg-row"><div><div class="pkg-name">${esc(p.id)}</div>
      <div class="pkg-meta"><span class="ver">${esc(p.version)}</span><span class="muted">· USB'de kurulu sürüm</span></div></div>
      <div class="pkg-actions"><button class="btn sm ghost" disabled title="Güncelleme denetimi bu taslakta yok">güncelleme denetimi yok</button></div></div>`;
  }

  function kpis() {
    const sv = home.servers, up = sv.filter(s => s.checked && s.up).length, chk = sv.filter(s => s.checked).length, keys = sv.filter(s => s.hasKey).length;
    const day = Date.now() - 864e5;
    const fails = sv.flatMap(s => (s.recent || []).filter(a => !isOk(a.result) && Date.parse(a.time) > day).map(a => ({ ...a, server: s.name })));
    const ly = sv.filter(s => s.lynis);
    const avg = ly.length ? Math.round(ly.reduce((t, s) => t + s.lynis.score, 0) / ly.length) : null;
    const gain = ly.reduce((t, s) => t + (s.lynis.score - s.lynis.history[0]), 0);
    return `
      <div class="kpi"><div class="kpi-label"><span class="dot ${chk && up === sv.length ? "ok" : "warn"}" aria-hidden="true"></span>Çevrimiçi</div>
        <div class="kpi-value">${sv.some(s => s.valid) && !chk ? "…" : up}<small>/${sv.length}</small></div>
        <div class="kpi-sub"><span class="ok">✔</span> anahtar var: ${keys} · <span class="muted">yok: ${sv.length - keys}</span></div></div>
      <div class="kpi"><div class="kpi-label"><span class="dot ok" aria-hidden="true"></span>Kurulu paket</div>
        <div class="kpi-value">${home.packages.length}</div>
        <div class="kpi-sub">${esc(home.packages.map(p => p.id).join(" · ")) || "—"}</div></div>
      <div class="kpi"><div class="kpi-label"><span class="dot ${fails.length ? "danger" : "ok"}" aria-hidden="true"></span>Son 24 sa başarısız</div>
        <div class="kpi-value">${fails.length}</div>
        <div class="kpi-sub">${fails.length ? `<span class="danger">${esc(fails[0].server)}</span> · ${esc(fails[0].package)} ${esc(fails[0].action)}` : "başarısız eylem yok"}</div></div>
      <div class="kpi"><div class="kpi-label"><span class="dot ${avg === null ? "warn" : "ok"}" aria-hidden="true"></span>Ortalama Lynis</div>
        <div class="kpi-value">${avg === null ? "—" : avg}<small>${avg === null ? "" : "/100"}</small></div>
        <div class="kpi-sub">${avg === null ? "ölçüm yok" : `${ly.length} sunucu · ${gain >= 0 ? "+" : ""}${gain} toplam değişim`}</div></div>`;
  }

  function pills() {
    const sv = home.servers, up = sv.filter(s => s.checked && s.up).length, off = sv.filter(s => s.checked && !s.up).length;
    const day = Date.now() - 864e5;
    const f = sv.reduce((t, s) => t + (s.recent || []).filter(a => !isOk(a.result) && Date.parse(a.time) > day).length, 0);
    return `${sv.some(s => s.valid && !s.checked) ? '<span class="pill"><span class="dot" aria-hidden="true"></span>denetleniyor…</span>' : ""}
      ${sv.some(s => s.checked) ? `<span class="pill ok"><span class="dot ok" aria-hidden="true"></span>${up} çevrimiçi</span>` : ""}
      ${off ? `<span class="pill warn"><span class="dot warn" aria-hidden="true"></span>${off} kapalı</span>` : ""}
      ${f ? `<span class="pill danger"><span class="dot danger" aria-hidden="true"></span>${f} başarısız (24 sa)</span>` : ""}`;
  }

  function spark(hist) {
    if (!hist || hist.length < 2) return "";
    const W = 200, H = 64, n = hist.length;
    const pts = hist.map((v, i) => [Math.round((i / (n - 1)) * W), Math.round(H - 4 - (Math.max(0, Math.min(100, v)) / 100) * (H - 8))]);
    const line = pts.map(p => p.join(",")).join(" ");
    const last = pts[n - 1];
    return `<svg class="spark" viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" role="img" aria-label="Lynis skoru: ${hist.join(", ")}">
      <g class="grid"><line x1="0" y1="16" x2="${W}" y2="16"/><line x1="0" y1="32" x2="${W}" y2="32"/><line x1="0" y1="48" x2="${W}" y2="48"/></g>
      <path class="area" d="M${pts.map(p => p.join(",")).join(" L")} L${W},${H} L0,${H} Z"/>
      <polyline class="line" points="${line}"/><circle class="pt last" cx="${last[0]}" cy="${last[1]}" r="3.5"/></svg>`;
  }

  function lynisCard(s, big) {
    if (!s) return '<h3>Lynis skoru</h3><div class="small muted">Sunucu seçilmedi.</div>';
    const l = s.lynis;
    if (!l) return `<h3>Lynis skoru — ${esc(s.name)}</h3><div class="small muted">Bu sunucu için ölçüm yok. Menüden linux-hardening <span class="mono">audit</span> çalıştırın.</div>`;
    const d = l.score - l.history[0];
    return `<h3>Lynis skoru — ${esc(s.name)}</h3>
      <div class="lynis-head"><div class="lynis-score">${l.score}</div><div class="small muted" style="align-self:flex-end;margin-bottom:4px">/100</div>
        ${l.history.length > 1 ? `<div class="lynis-delta">${d >= 0 ? "▲ +" : "▼ "}${d}</div>` : ""}</div>
      <div class="lynis-meta"><span class="badge warn">uyarı ${l.warnings}</span><span class="badge info">öneri ${l.suggestions}</span>
        ${l.history.length > 1 ? `<span class="badge plain">${l.history[0]} → ${l.score}</span>` : ""}</div>
      ${big ? `<div class="chartbox">${chart(l)}</div>` : spark(l.history)}
      <div class="small muted" style="margin-top:6px;display:flex;justify-content:space-between"><span>${l.history.length} ölçüm</span><span>${esc((l.date || "").slice(0, 10))}</span></div>`;
  }

  function timeline() {
    const all = home.servers.flatMap(s => (s.recent || []).map(a => ({ ...a, server: s.name })))
      .sort((a, b) => Date.parse(b.time) - Date.parse(a.time)).slice(0, 7);
    if (!all.length) return '<li class="small muted">Henüz kayıt yok.</li>';
    return all.map(a => `<li class="${isOk(a.result) ? "ok" : "fail"}"><div class="tl-time" title="${esc(localTime(a.time))}">${esc(ago(a.time))} önce</div>
      <div class="tl-text"><b>${esc(a.server)}</b> · ${esc(a.package)} ${esc(a.version)} · ${esc(a.action)} · <span style="color:var(--${isOk(a.result) ? "ok" : "danger"})">${isOk(a.result) ? "başarılı" : "başarısız"}</span></div></li>`).join("");
  }

  /* Sunucu sayfasının ortadaki sunucu listesi (Proxmox tarzı kaynak ağacı) */
  let treeQ = "";
  function renderSrvTree() {
    const el = $("#srvtree-list"); if (!el) return;
    const sv = home.servers.filter(s => !treeQ || `${s.name} ${s.host || ""}`.toLowerCase().includes(treeQ));
    $("#srvtree-title").textContent = `${t("tree_title", "Sunucular")} (${home.servers.length})`;
    el.innerHTML = sv.map(s => `<li><button class="tree-item${s.name === srvName ? " on" : ""}" data-tsrv="${esc(s.name)}" aria-current="${s.name === srvName ? "page" : "false"}"><span class="status-dot ${dotState(s)}" aria-label="durum: ${stateText(s)}"></span><span class="tree-txt"><span class="tree-name">${esc(s.name)}</span><span class="tree-addr mono">${esc(s.host || "")}</span></span></button></li>`).join("") || '<li class="small muted" style="padding:8px 10px">${t("tree_none", "Eşleşen sunucu yok.")}</li>';
  }
  $("#srvtree-list").addEventListener("click", e => { const b = e.target.closest("[data-tsrv]"); if (b && b.dataset.tsrv !== srvName) openServer(b.dataset.tsrv, srvTab); });
  $("#srvtree-q").addEventListener("input", e => { treeQ = e.target.value.trim().toLowerCase(); renderSrvTree(); });
  $("#srvtree-add").addEventListener("click", () => showView("add"));
  /* Dil: düğme TR/EN arasında geçer; dinamik çizimler yeniden yapılır */
  $("#lang-btn").addEventListener("click", () => { const L = window.KeyLabI18n; L.setLang(L.getLang() === "tr" ? "en" : "tr"); });
  window.addEventListener("keylab:langchange", () => { render(); renderSrvTree(); if (view === "server" && srv) renderServer(); });

  function render() {
    renderSrvTree();
    const sv = home.servers.filter(s => matches(s.name, s.platform, s.last && s.last.package));
    const pk = home.packages.filter(p => matches(p.id));
    const addTile = '<li class="stile add" tabindex="0" data-add="1" aria-label="Sunucu ekle"><div class="plus" aria-hidden="true">+</div><div>Sunucu ekle</div></li>';
    const rows = (sv.map(mode === "tiles" ? tile : serverRow).join("") || emptyServers) + (mode === "tiles" && !query ? addTile : "");
    for (const id of ["#servers", "#servers-all"]) { $(id).innerHTML = rows; $(id).classList.toggle("tiles", mode === "tiles"); }
    $$(".mode-toggle button").forEach(b => b.setAttribute("aria-pressed", String(b.dataset.mode === mode)));
    $("#servers-count").textContent = home.servers.length + " kayıt";
    $("#pkgs-dash").innerHTML = pk.map(pkgRow).join("") || '<div class="small muted" style="padding:12px">Paket bulunamadı.</div>';
    $("#kpis").innerHTML = kpis();
    $("#pills").innerHTML = pills();
    $("#n-servers").textContent = home.servers.length;
    $("#n-packages").textContent = home.packages.length;
    $("#idx-serial").textContent = "seri: " + (home.indexSerial || "—");
    if (!selected || !home.servers.some(s => s.name === selected)) {
      const measured = home.servers.filter(s => s.lynis).sort((a, b) => (b.lynis.date || "").localeCompare(a.lynis.date || ""));
      const first = measured[0] || home.servers[0];
      selected = first ? first.name : "";
    }
    $("#a-lynis").innerHTML = lynisCard(home.servers.find(s => s.name === selected));
    $("#a-tl").innerHTML = timeline();
    bindServers();
    $("#brand-sub").textContent = "yerel · taslak";
  }

  async function load() {
    try { home = await json("/api/home"); home.servers = home.servers || []; home.packages = home.packages || []; render(); }
    catch (e) { $("#brand-sub").textContent = "hata: " + e.message; return; }
    refreshStatus();
  }
  // erişilebilirlik ölçümü (kapalı sunucu 1,5 sn bekletir): sayfa çizildikten sonra arkadan gelir
  let statusBusy = false;
  async function refreshStatus() {
    if (statusBusy) return;
    statusBusy = true;
    try {
      const st = await json("/api/status?detail=1");
      home.servers.forEach(s => { if (s.name in st) { s.up = st[s.name].up; s.conn = st[s.name].conn; s.checked = true; } });
      render();
    } catch (e) { /* bir sonraki turda yeniden denenir */ }
    statusBusy = false;
  }

  /* ---------------- Değişken düzenleme ---------------- */
  const varsm = $("#varsmodal");
  let vData = [], vSrv = "", vPkg = "", vQ = "", vOnlySet = false, vGroup = "";

  async function openVars(server, pkg, group) {
    vSrv = server; vPkg = pkg; vQ = ""; vOnlySet = false; vGroup = group || ""; // group: yalnız bu gruptaki değişkenler (uygulama kartı)
    $("#vars-q").value = ""; $("#vars-set").checked = false;
    $("#vars-title").textContent = "Değişkenler · " + (TAB_LABEL[pkg] || pkg) + (vGroup ? " · " + vGroup : "");
    $("#vars-sub").textContent = `Sunucu: ${server} · servers/${server}/${pkg}.vars`;
    $("#vars-list").innerHTML = '<div class="small muted">yükleniyor…</div>';
    varsm.classList.add("open");
    await loadVars();
  }
  async function loadVars() {
    try { vData = (await json(`/api/vars?server=${encodeURIComponent(vSrv)}&package=${encodeURIComponent(vPkg)}`)).vars || []; }
    catch (e) { $("#vars-list").innerHTML = `<div class="notice danger"><span class="ico" aria-hidden="true">✕</span><div>Yüklenemedi: ${esc(e.message)}</div></div>`; return; }
    paintVars();
  }
  const varCur = v => (v.set ? v.value : v.default);

  function varControl(v) {
    const cur = varCur(v);
    if (v.type === "bool") return `<select data-ctl>${["true", "false"].map(o => `<option value="${o}"${String(cur).toLowerCase() === o ? " selected" : ""}>${o}</option>`).join("")}</select>`;
    if (v.values && v.values.length) return `<select data-ctl>${v.values.map(o => `<option value="${esc(o)}"${o === cur ? " selected" : ""}>${esc(o)}</option>`).join("")}</select>`;
    if (/^(int|integer|number|float)$/.test(v.type)) return `<input data-ctl type="number" value="${esc(cur)}"${v.min !== "" && v.min != null ? ` min="${esc(v.min)}"` : ""}${v.max !== "" && v.max != null ? ` max="${esc(v.max)}"` : ""} step="any">`;
    return `<input data-ctl type="text" value="${esc(cur)}" placeholder="${esc(v.default)}" spellcheck="false" autocomplete="off"${/^list/.test(v.type) ? ' aria-describedby="vl"' : ""}>`;
  }

  function paintVars() {
    const q = vQ.toLowerCase(), scope = vData.filter(v => !vGroup || v.group === vGroup);
    const list = scope.filter(v => (!vOnlySet || v.set) && (!q || `${v.name} ${v.description} ${v.group}`.toLowerCase().includes(q)));
    $("#vars-count").textContent = `${list.length} / ${scope.length}`;
    const groups = {};
    list.forEach(v => (groups[v.group || "Diğer"] = groups[v.group || "Diğer"] || []).push(v));
    const row = v => `<div class="vrow" data-n="${esc(v.name)}">
      <div><div class="vname">${esc(v.name)} ${v.set ? '<span class="badge ok">ayarlı</span>' : v.required && !v.default ? '<span class="badge danger">gerekli</span>' : '<span class="badge plain">varsayılan</span>'} <span class="chip" style="font-size:11px">${esc(v.type)}</span></div>
        <div class="vdesc small muted" title="Tamamını görmek için tıklayın">${esc(v.description)}</div>
        ${v.warnIf ? `<div class="small" style="margin-top:4px;color:var(--warn)">⚠ uyarı eşiği: ${esc(v.warnIf)}</div>` : ""}</div>
      <div class="vctl">${varControl(v)}
        <div class="row-flex"><button class="btn sm primary" data-save disabled>Kaydet</button>${v.set ? '<button class="btn sm ghost" data-reset title="Satırı siler; paket varsayılanına döner">Varsayılana dön</button>' : ""}<span class="small muted">${v.default !== "" ? `varsayılan: <span class="mono">${esc(v.default)}</span>` : "varsayılan yok"}</span></div>
        <div class="vmsg" role="status"></div></div></div>`;
    $("#vars-list").innerHTML = Object.entries(groups).map(([g, vs]) => `<div class="vgroup">${esc(g)}</div>${vs.map(row).join("")}`).join("") || '<div class="small muted">Eşleşen değişken yok.</div>';
    $$("#vars-list .vrow").forEach(rowEl => bindVarRow(rowEl));
  }

  function bindVarRow(rowEl) {
    const v = vData.find(x => x.name === rowEl.dataset.n), ctl = $("[data-ctl]", rowEl), save = $("[data-save]", rowEl), msg = $(".vmsg", rowEl);
    const say = (cls, t) => { msg.className = "vmsg " + cls; msg.textContent = t; };
    let ack = false;
    $(".vdesc", rowEl).addEventListener("click", e => e.currentTarget.classList.toggle("open"));
    const dirty = () => { const d = ctl.value !== varCur(v); rowEl.classList.toggle("dirty", d); save.disabled = !d; ack = false; save.textContent = "Kaydet"; if (d) say("", ""); };
    ctl.addEventListener("input", dirty); ctl.addEventListener("change", dirty);
    ctl.addEventListener("keydown", e => { if (e.key === "Enter" && !save.disabled) { e.preventDefault(); save.click(); } });
    const post = async (action, value) => {
      const r = await fetch("/api/vars", { method: "POST", headers: { "X-Requested-With": "sshpocket" }, body: JSON.stringify({ server: vSrv, package: vPkg, name: v.name, value, action, ack }) });
      return { ok: r.ok, status: r.status, body: await r.json().catch(() => ({})) };
    };
    const done = async text => {
      toast(text); catSrv = ""; // katalog kartlarındaki değerler yeniden okunsun
      try { srv = await json("/api/server?name=" + encodeURIComponent(vSrv)); if (view === "server") renderServer(); } catch (e) { /* bir sonraki turda */ }
      const top = $("#vars-list").parentElement.scrollTop; await loadVars(); $("#vars-list").parentElement.scrollTop = top;
    };
    save.addEventListener("click", async () => {
      save.disabled = true;
      const r = await post("set", ctl.value);
      if (r.ok) return done(`${v.name} kaydedildi`);
      if (r.status === 409 && r.body.needAck) { ack = true; save.disabled = false; save.textContent = "Onayla ve kaydet"; return say("warn", "⚠ " + String(r.body.error).replace(/[.;\s]+$/, "") + ". Yine de kaydetmek için tekrar tıklayın."); }
      save.disabled = false; say("err", "✖ " + (r.body.error || "kaydedilemedi"));
    });
    const reset = $("[data-reset]", rowEl);
    if (reset) reset.addEventListener("click", async () => { reset.disabled = true; const r = await post("unset", ""); if (r.ok) return done(`${v.name} varsayılana döndü`); reset.disabled = false; say("err", "✖ " + (r.body.error || "sıfırlanamadı")); });
  }
  $("#vars-q").addEventListener("input", e => { vQ = e.target.value.trim(); paintVars(); });
  $("#vars-set").addEventListener("change", e => { vOnlySet = e.target.checked; paintVars(); });
  const closeVars = () => varsm.classList.remove("open");
  $("#vars-x").addEventListener("click", closeVars); $("#vars-close").addEventListener("click", closeVars);

  /* ---------------- Eylem çalıştırma (salt okunur eylemler) ---------------- */
  const runm = $("#runmodal");
  let runJob = "", runPoll = 0, runTick = 0, runStart = 0, runSrv = "", runPkg = "", runAct = "";
  const runBusy = () => !!runJob;

  let runMode = "exec", termTimer = 0;
  function openRun(server, pkg, action) {
    runMode = "exec"; $("#run-log").hidden = false; $("#run-cancel").hidden = false;
    $("#run-yes").hidden = $("#run-no").hidden = true;
    $("#run-confirm-extra").hidden = true; $("#run-confirm-extra").innerHTML = ""; $("#run-go").classList.remove("warnbtn", "dangerbtn"); $("#run-ext").hidden = true; $("#run-ext").disabled = false; $("#run-input").hidden = true;
    runSrv = server; runPkg = pkg; runAct = action; runJob = "";
    $("#run-title").innerHTML = `<span class="mono">${esc(action)}</span> · ${esc(TAB_LABEL[pkg] || pkg)}`;
    $("#run-confirm-note").textContent = "Çalıştırma uygulamanın kendi motoruyla yapılır; kapılar, günlük ve sonuç kaydı menüdekiyle aynıdır. Parola sorulmaz: ssh anahtarınız ssh-agent'ta yüklü olmalı.";
    $("#run-sub").textContent = `Sunucu: ${server}`;
    $("#run-confirm-text").innerHTML = `<b>${esc(server)}</b> sunucusuna ssh ile bağlanılacak ve <span class="mono">${esc(action)}</span> <b>salt okunur</b> çalıştırılacak: sunucuda hiçbir şey değişmez.`;
    $("#run-confirm-note").textContent = "Çalıştırma uygulamanın kendi motoruyla yapılır; kapılar, günlük ve sonuç kaydı menüdekiyle aynıdır. Parola sorulmaz: ssh anahtarınız ssh-agent'ta yüklü olmalı.";
    $("#run-confirm").hidden = false; $("#run-live").hidden = true;
    $("#run-go").hidden = false; $("#run-go").disabled = false; $("#run-go").textContent = "Çalıştır";
    $("#run-close").hidden = false; $("#run-close").textContent = "Vazgeç";
    runm.classList.add("open"); setTimeout(() => $("#run-go").focus(), 40);
  }
  // paketin bu sunucudaki son çalıştırma günlüğünü Günlükler sayfasında aç
  async function openLastLog(pkg, server) {
    showView("logs"); logSrv = server; logPkg = pkg; logKind = "last"; logQ = "";
    await loadLogs();
    const l = logAll.find(x => x.package === pkg && x.server === server && x.file === "last-run.log");
    if (l) { const i = $$("#loglist button[data-i]").findIndex(b => b.textContent.includes("last-run.log")); await openLog(l, $$("#loglist button[data-i]")[i]); }
  }
  function closeRun() { if (runBusy()) return; clearInterval(termTimer); termTimer = 0; runm.classList.remove("open"); }

  /* değişiklik yapan eylem: plan → snapshot + onay → Terminal (sudo parolası orada) → canlı ilerleme.
     Motor Terminal'de aynı onayları yeniden ister (tek doğruluk kaynağı); arayüz onayı bunun ÖN adımıdır. */
  const EFFECT_TXT = { "installs-package": "paket kurar", "changes-config": "yapılandırmayı değiştirir", destructive: "yıkıcı" };
  let wAct = null, logSince = 0;
  function openWrite(server, pkg, action) {
    openRun(server, pkg, action);
    runMode = "write";
    const p = srv.packages.find(x => x.id === pkg), a = actionOf(pkg, action); wAct = a;
    const pv = (p.vars || []).filter(v => v.set);
    const prereq = (a.prereqs || []).map(q => {
      const act = p.actions.find(x => x.id === q.action);
      const chips = act && act.keys ? Object.entries(act.keys).map(([k, v]) => `<span class="chip">${esc(k)}=${esc(v)}</span>`).join("") : "";
      return `<div style="margin-top:6px"><span class="ok">✔ ${esc(q.action)}</span> <span class="muted small">${Math.max(1, Math.round(q.ageSec / 60))} dk önce</span>${chips ? `<div class="chips">${chips}</div>` : ""}</div>`;
    }).join("");
    const warns = (a.warnings || []);
    const destr = !!a.destructive;
    const typeName = destr || a.confirm === "type-server-name";
    $("#run-title").innerHTML = `<span class="mono">${esc(action)}</span> · ${esc(TAB_LABEL[pkg] || pkg)} <span class="draft-tag" style="background:var(--danger-weak);color:var(--danger);border-color:var(--danger-border)">${destr ? "YIKICI" : "yazar"}</span>`;
    $("#run-confirm-text").innerHTML = destr
      ? `<b>⚠ YIKICI EYLEM.</b> <b>${esc(server)}</b> sunucusunda dosya/veri <b>silinebilir ya da değiştirilebilir ve geri alınamayabilir</b>: ${(a.effects || []).map(e => esc(EFFECT_TXT[e] || e)).join(", ")}. Aşağıdaki uyarıların tamamını okuyun.`
      : `<b>${esc(server)}</b> sunucusuna <b>gerçekten yazılacak</b>: ${(a.effects || []).map(e => esc(EFFECT_TXT[e] || e)).join(", ")}.`;
    $("#run-confirm-extra").hidden = false;
    $("#run-confirm-extra").innerHTML = `
      ${prereq ? `<div class="small muted" style="margin-top:12px">Önkoşul (motor 30 dk tazelik ister)</div>${prereq}` : ""}
      ${pv.length ? `<div class="small muted" style="margin-top:12px">Bu değişkenlerle</div><div class="chips">${pv.map(v => `<span class="chip">${esc(v.name)}=${esc(v.value.length > 48 ? v.value.slice(0, 48) + "…" : v.value)}</span>`).join("")}</div>` : ""}
      ${warns.length ? `<div class="small muted" style="margin-top:12px">Paketin uyarıları${destr ? " (tamamı)" : ""}</div><ul class="small" style="margin:4px 0 0;padding-left:18px;color:var(--text-2);line-height:1.5">${warns.slice(0, destr ? warns.length : 3).map(w => `<li>${esc(w)}</li>`).join("")}</ul>${!destr && warns.length > 3 ? `<details class="small" style="margin-top:4px"><summary class="muted" style="cursor:pointer">${warns.length - 3} uyarı daha</summary><ul style="padding-left:18px">${warns.slice(3).map(w => `<li>${esc(w)}</li>`).join("")}</ul></details>` : ""}` : ""}
      <label class="small" style="display:flex;gap:8px;align-items:flex-start;margin-top:14px"><input type="checkbox" id="w-snap"> <span><b>VM snapshot'ı aldım</b>; sorun çıkarsa geri dönebilirim.</span></label>
      ${destr ? `<label class="small" style="display:flex;gap:8px;align-items:flex-start;margin-top:10px;color:var(--danger)"><input type="checkbox" id="w-destr"> <span><b>Bu eylemin geri alınamayabileceğini anlıyorum</b>; yedeğim var ve sonuçlarını üstleniyorum.</span></label>` : ""}
      ${typeName ? `<div class="field" style="margin-top:12px"><label for="w-confirm">Onay için sunucu adını yazın: <span class="mono">${esc(server)}</span></label><input id="w-confirm" autocomplete="off" spellcheck="false" placeholder="${esc(server)}"><div class="hint">Büyük/küçük harf duyarlıdır.</div></div>`
        : `<label class="small" style="display:flex;gap:8px;align-items:flex-start;margin-top:10px"><input type="checkbox" id="w-yes"> <span>Evet, bu eylemi <b>${esc(server)}</b> üzerinde çalıştırmak istiyorum.</span></label>`}`;
    $("#run-confirm-note").textContent = "Motor aynı onayları (snapshot, e/h ya da ad) ve sudo parolasını çıktıda yeniden sorabilir: kurallar menüdekiyle aynıdır, yanıtları burada verirsiniz. Çalışırken ilerleme izlenir. Parolayı web sayfasına yazmak istemezseniz \"macOS Terminal'de aç\" düğmesini kullanın.";
    $("#run-go").textContent = destr ? "Yıkıcı eylemi çalıştır" : "Çalıştır (onaylı)"; $("#run-go").classList.add(destr ? "dangerbtn" : "warnbtn"); $("#run-go").disabled = true; $("#run-ext").hidden = false;
    const check = () => { const okc = $("#w-snap").checked && (!destr || $("#w-destr").checked) && (typeName ? $("#w-confirm").value === server : $("#w-yes").checked); $("#run-go").disabled = !okc; $("#run-ext").disabled = !okc; };
    $("#run-ext").disabled = true;
    ["#w-snap", "#w-destr", "#w-confirm", "#w-yes"].forEach(id => { const e = $(id); if (e) { e.addEventListener("input", check); e.addEventListener("change", check); } });
  }

  // last-run.log'u izle: SSHPOCKET_PROGRESS satırlarından alan alan ilerleme, geri alma güvenlik ağı durumu
  async function pollProgress(pkg, server, since) {
    try {
      const q = new URLSearchParams({ package: pkg, server, file: "last-run.log" });
      const r = await api("/api/log?" + q);
      const m = Date.parse(r.headers.get("X-Log-Mtime") || "");
      if (!m || m < since - 3000) return; // önceki çalıştırmanın günlüğü: yok say
      const txt = await r.text();
      const prog = [...txt.matchAll(/SSHPOCKET_PROGRESS alan=(\S+) sira=(\d+)\/(\d+) durum=(\S+)/g)];
      let total = 0, done = new Set(), cur = "";
      prog.forEach(x => { total = +x[3]; if (x[4] === "bitti") done.add(x[1]); else cur = x[1]; });
      if (done.has(cur)) cur = "";
      const armed = /TASK \[hardening_rollback : Arm[^\]]*\][^\n]*\n(ok|changed):/.test(txt), disarmed = /TASK \[hardening_rollback : Disarm[^\]]*\][^\n]*\n(ok|changed):/.test(txt);
      const tail = txt.trimEnd().split("\n").slice(-6).map(esc).join("\n");
      $("#run-result").innerHTML = (total ? `<div class="run-meta"><span>${done.size} / ${total} alan tamam${cur ? ` · şu an: <b>${esc(cur)}</b>` : ""}</span></div>
          <div class="progress" role="progressbar" aria-valuemin="0" aria-valuemax="${total}" aria-valuenow="${done.size}"><i style="width:${Math.round(done.size / total * 100)}%"></i></div>` : '<div class="small muted">Motor başlıyor…</div>') +
        (armed && !disarmed ? '<div class="notice" style="margin-top:10px"><span class="ico" aria-hidden="true">⏱</span><div><b>Geri alma güvenlik ağı devrede.</b> Erişim doğrulanınca kaldırılır; doğrulanamazsa önceki hâl geri yüklenir.</div></div>' : "") +
        (disarmed ? '<div class="small muted" style="margin-top:8px">Geri alma güvenlik ağı kaldırıldı (erişim doğrulandı).</div>' : "") +
        `<pre class="mono small" style="margin:10px 0 0;white-space:pre-wrap;color:var(--text-2)">${tail}</pre>`;
    } catch (e) { /* günlük henüz yok ya da okunamadı: bir sonraki turda */ }
  }

  /* sudo gerektiren salt okunur eylem: macOS Terminal'de sshpocket motoru; parolayı kullanıcı orada girer, arayüz görmez */
  function actionOf(pkg, action) { const p = srv && srv.packages.find(x => x.id === pkg); return p && p.actions.find(x => x.id === action); }
  function openTerm(server, pkg, action) {
    openRun(server, pkg, action);
    runMode = "term";
    $("#run-confirm-text").innerHTML = `<b>${esc(server)}</b> sunucusunda <span class="mono">${esc(action)}</span> (<b>salt okunur</b>) çalışacak. Motor <b>sudo parolası</b> isteyecek: bu pencerede çıkan parola kutusuna yazarsınız; doğrudan Ansible'a iletilir, arayüz onu saklamaz ya da günlüğe yazmaz.`;
    $("#run-go").textContent = "Çalıştır"; $("#run-ext").hidden = false;
    const pv = (actionOf(pkg, action) && srv && srv.packages.find(x => x.id === pkg).vars || []).filter(v => v.set);
    if (pv.length) $("#run-confirm-text").insertAdjacentHTML("beforeend", `<div class="chips" style="margin-top:8px"><span class="small muted" style="align-self:center">Bu değişkenlerle:</span>${pv.map(v => `<span class="chip">${esc(v.name)}=${esc(v.value.length > 48 ? v.value.slice(0, 48) + "…" : v.value)}</span>`).join("")}</div>`);
    $("#run-confirm-note").textContent = "Parola istemi çıktıda belirince burada bir kutu açılır. Sonuç menüdekiyle aynı şekilde kaydedilir. ssh anahtarınızın ssh-agent'ta yüklü olması gerekir. Parolayı bir web sayfasına yazmak istemezseniz \"macOS Terminal'de aç\" düğmesini kullanın.";
  }
  async function startTerm() {
    $("#run-go").disabled = true; $("#run-ext").hidden = true;
    const a0 = actionOf(runPkg, runAct), base = a0 ? a0.lastTime || "" : "";
    const body = { server: runSrv, package: runPkg, action: runAct };
    if (runMode === "write") { body.snapshot = $("#w-snap").checked; body.ackDestructive = !!($("#w-destr") && $("#w-destr").checked); body.confirm = $("#w-confirm") ? $("#w-confirm").value : $("#w-yes").checked ? "evet" : ""; }
    logSince = Date.now();
    try { await json("/api/run-terminal", body); }
    catch (e) { $("#run-go").disabled = false; $("#run-ext").hidden = false; toast("Terminal açılamadı: " + e.message); return; }
    $("#run-confirm").hidden = true; $("#run-live").hidden = false; $("#run-go").hidden = true; $("#run-close").hidden = false; $("#run-close").textContent = "Kapat";
    $("#run-log").hidden = true; $("#run-cancel").hidden = true; $("#run-result").innerHTML = "";
    const t0 = Date.now();
    const tick = () => { $("#run-state").innerHTML = `<span class="spinner" aria-hidden="true"></span> Terminal'de çalışıyor… parolayı orada girin · ${Math.round((Date.now() - t0) / 1000)} sn`; };
    tick();
    clearInterval(termTimer);
    let prgTimer = 0;
    if (runMode === "write") { prgTimer = setInterval(() => pollProgress(runPkg, runSrv, logSince), 2000); }
    termTimer = setInterval(async () => {
      tick();
      if (Date.now() - t0 > (runMode === "write" ? 40 : 10) * 60 * 1000) { clearInterval(termTimer); clearInterval(prgTimer); $("#run-state").textContent = "Sonuç beklenirken süre doldu; sayfa yine de sonucu gösterecek."; return; }
      try {
        const r = await json("/api/server?name=" + encodeURIComponent(runSrv));
        const a1 = r.packages.find(x => x.id === runPkg)?.actions.find(x => x.id === runAct);
        if (a1 && (a1.lastTime || "") !== base) {
          clearInterval(termTimer); termTimer = 0; clearInterval(prgTimer); srv = r; if (view === "server") renderServer(); load();
          $("#run-state").innerHTML = a1.lastResult === "ok" ? '<span class="res ok">✔ tamamlandı</span> · sonuç kartta güncellendi' : '<span class="res fail">✖ başarısız</span> · ayrıntı için Terminal penceresine ve Günlükler sayfasına bakın';
          $("#run-result").innerHTML = (a1.keys ? `<div class="chips">${Object.entries(a1.keys).map(([k, v]) => `<span class="chip">${esc(k)}=${esc(v)}</span>`).join("")}</div>` : "") +
            `<div style="margin-top:10px"><button class="btn sm" id="run-open-log">Çıktıyı aç (son çalıştırma günlüğü)</button></div>`;
          $("#run-open-log").addEventListener("click", () => { const pk = runPkg, sv = runSrv; closeRun(); openLastLog(pk, sv); });
        }
      } catch (e) { /* bir sonraki turda yeniden denenir */ }
    }, 3000);
  }

  // ham terminal çıktısı → okunur metin: ANSI kaçışlarını at, "\r" ile satır başına dönmeyi uygula
  function cleanTerm(t) {
    t = t.replace(/\x1b\[[0-9;?]*[ -\/]*[@-~]/g, "").replace(/\x1b\][^\x07]*\x07/g, "").replace(/\^D|\x08/g, "");
    return t.split("\n").map(l => { let out = ""; for (const p of l.split("\r")) if (p.length) out = p + out.slice(p.length); return out; }).join("\n");
  }
  const lineHtml = l => { const c = /fatal:|FAILED|failed=[1-9]|HATA|ERROR/.test(l) ? "err" : /^ok:|^changed:/.test(l) ? "ok" : /^TASK |^PLAY /.test(l) ? "t" : ""; return `<div${c ? ` class="${c}"` : ""}>${esc(l) || " "}</div>`; };

  // SSHPOCKET_PROGRESS satırlarından alan alan ilerleme + geri alma güvenlik ağı durumu
  function progressHtml(txt) {
    const prog = [...txt.matchAll(/SSHPOCKET_PROGRESS alan=(\S+) sira=(\d+)\/(\d+) durum=(\S+)/g)];
    if (!prog.length) return "";
    let total = 0, done = new Set(), cur = "";
    prog.forEach(x => { total = +x[3]; if (x[4] === "bitti") done.add(x[1]); else cur = x[1]; });
    if (done.has(cur)) cur = "";
    const armed = /TASK \[hardening_rollback : Arm[^\]]*\][^\n]*\n(ok|changed):/.test(txt), disarmed = /TASK \[hardening_rollback : Disarm[^\]]*\][^\n]*\n(ok|changed):/.test(txt);
    return `<div class="run-meta"><span>${done.size} / ${total} alan tamam${cur ? ` · şu an: <b>${esc(cur)}</b>` : ""}</span></div>
      <div class="progress" role="progressbar" aria-valuemin="0" aria-valuemax="${total}" aria-valuenow="${done.size}"><i style="width:${Math.round(done.size / total * 100)}%"></i></div>` +
      (armed && !disarmed ? '<div class="notice" style="margin-top:10px"><span class="ico" aria-hidden="true">⏱</span><div><b>Geri alma güvenlik ağı devrede.</b> Erişim doğrulanınca kaldırılır; doğrulanamazsa önceki hâl geri yüklenir.</div></div>' : "") +
      (disarmed ? '<div class="small muted" style="margin-top:8px">Geri alma güvenlik ağı kaldırıldı (erişim doğrulandı).</div>' : "");
  }

  async function startRun() {
    $("#run-go").disabled = true; askedAt = askCancelledAt = -1; askBusy = false;
    const body = { server: runSrv, package: runPkg, action: runAct };
    if (runMode === "write") { body.snapshot = $("#w-snap").checked; body.ackDestructive = !!($("#w-destr") && $("#w-destr").checked); body.confirm = $("#w-confirm") ? $("#w-confirm").value : $("#w-yes").checked ? "evet" : ""; }
    let r;
    try { r = runMode === "upgrade" ? await json("/api/upgrade", { package: runPkg, version: runUpgTo, ack: true }) : runMode === "task" ? await json("/api/server-task", { server: runSrv, task: runAct }) : await json("/api/run", body); }
    catch (e) { $("#run-go").disabled = false; toast("Çalıştırılamadı: " + e.message); return; }
    watchJob(r.id);
  }

  function watchJob(id) {
    runJob = id; runStart = Date.now();
    $("#run-confirm").hidden = true; $("#run-live").hidden = false; $("#run-go").hidden = true; $("#run-ext").hidden = true; $("#run-close").hidden = true;
    $("#run-log").innerHTML = ""; $("#run-log").hidden = false; $("#run-progress").innerHTML = ""; $("#run-result").innerHTML = ""; $("#run-input").hidden = true;
    $("#run-cancel").hidden = false; $("#run-cancel").disabled = false;
    let from = 0, raw = "", idle = 0;
    const logEl = $("#run-log");
    const tick = () => { $("#run-state").innerHTML = `<span class="spinner" aria-hidden="true"></span> çalışıyor… ${Math.round((Date.now() - runStart) / 1000)} sn`; };
    tick(); runTick = setInterval(tick, 1000);
    const poll = async () => {
      try {
        const d = await json(`/api/run?id=${encodeURIComponent(runJob)}&from=${from}`);
        if (d.text) {
          raw += d.text; idle = 0;
          const stick = logEl.scrollTop + logEl.clientHeight >= logEl.scrollHeight - 30;
          const txt = cleanTerm(raw);
          logEl.innerHTML = txt.split("\n").map(lineHtml).join("");
          if (stick) logEl.scrollTop = logEl.scrollHeight;
          $("#run-progress").innerHTML = progressHtml(txt);
        } else idle++;
        from = d.next;
        if (d.done) { $("#run-input").hidden = true; finishRun(d, cleanTerm(raw)); return; }
        // istem: çıktı satır sonuyla bitmiyor ve bir süredir yeni bayt gelmedi → girdi kutusunu göster
        const txt = cleanTerm(raw);
        if (idle >= 2 && raw && !/\n$/.test(raw.replace(/\r/g, ""))) {
          const prompt = txt.slice(txt.lastIndexOf("\n") + 1).trim();
          if (prompt) showInput(prompt, raw.length);
        } else if (idle < 2) $("#run-input").hidden = true;
      } catch (e) { clearInterval(runTick); runJob = ""; $("#run-state").textContent = "Bağlantı koptu: " + e.message; $("#run-close").hidden = false; $("#run-close").textContent = "Kapat"; return; }
      runPoll = setTimeout(poll, 600);
    };
    poll();
  }

  // parola girişi: "system" = işletim sisteminin kendi penceresi (tarayıcıya hiç uğramaz), "inline" = sayfa içi kutu
  let askMode = "system", askAvail = false, askBusy = false, askedAt = -1, askCancelledAt = -1;
  try { askMode = localStorage.getItem("sshpocket.askpass") === "inline" ? "inline" : "system"; } catch (e) { /* depolama kapalı olabilir */ }
  const maskSupported = typeof CSS !== "undefined" && CSS.supports && CSS.supports("-webkit-text-security", "disc");

  async function askNative(prompt, rawLen) {
    askBusy = true; $("#run-input").hidden = true;
    $("#run-state").innerHTML = '<span class="spinner" aria-hidden="true"></span> parola için sistem penceresi açıldı…';
    try { await json("/api/run/askpass", { id: runJob, prompt }); askedAt = rawLen; }
    catch (e) { askCancelledAt = rawLen; toast("Parola penceresi: " + e.message + " — sayfa içi kutu açıldı"); }
    askBusy = false;
  }

  // motor bir istem bekliyor (parola, e/h, ad onayı): kullanıcı yanıtı yazar
  function showInput(prompt, rawLen) {
    const secret = /pass(word|phrase)|parola|şifre|sifre/i.test(prompt);
    if (secret && askMode === "system" && askAvail && askCancelledAt !== rawLen) {
      if (!askBusy && askedAt !== rawLen) askNative(prompt, rawLen);   // aynı çıktı uzunluğunda yeniden sorma; yanlış parolada Ansible yeni çıktı üretir
      return;
    }
    const box = $("#run-input"), inp = $("#run-in");
    if (!box.hidden && $("#run-prompt").textContent === prompt) return;
    $("#run-prompt").textContent = prompt;
    const qa = /\[e\/h\]|\(e\/h\)/i.test(prompt) ? ["e", "H"] : /\(yes\/no/i.test(prompt) ? ["yes", "no"] : /\(y\/n\)|\[y\/n\]/i.test(prompt) ? ["y", "n"] : null;
    $("#run-yes").hidden = $("#run-no").hidden = !qa;
    if (qa) { $("#run-yes").dataset.v = qa[0]; $("#run-no").dataset.v = qa[1]; $("#run-yes").textContent = `Evet (${qa[0]})`; $("#run-no").textContent = `Hayır (${qa[1]})`; }
    // sayfa içi parola kutusu: Chrome'un şifre önerisini/parola yöneticisini tetiklememek için type=text + noktalı maske (destek yoksa password)
    inp.classList.toggle("mask-disc", secret && maskSupported);
    inp.type = secret && !maskSupported ? "password" : "text"; inp.value = "";
    inp.setAttribute("autocomplete", "off"); inp.setAttribute("name", "yanit-" + Math.random().toString(36).slice(2)); inp.setAttribute("data-lpignore", "true"); inp.setAttribute("data-1p-ignore", "true");
    box.hidden = false; inp.focus();
  }
  async function sendInput() {
    const inp = $("#run-in"), text = inp.value;
    inp.value = ""; $("#run-input").hidden = true;      // gönderince hemen temizle ve gizle
    try { await json("/api/run/input", { id: runJob, text }); }
    catch (e) { toast("Gönderilemedi: " + e.message); }
  }
  $("#run-send").addEventListener("click", sendInput);
  const quick = async t => { $("#run-input").hidden = true; try { await json("/api/run/input", { id: runJob, text: t }); } catch (e) { toast("Gönderilemedi: " + e.message); } };
  $("#run-yes").addEventListener("click", () => quick($("#run-yes").dataset.v || "e")); $("#run-no").addEventListener("click", () => quick($("#run-no").dataset.v || "H"));
  $("#run-in").addEventListener("keydown", e => { if (e.key === "Enter") { e.preventDefault(); sendInput(); } });

  async function finishRun(d, txt) {
    clearInterval(runTick); clearTimeout(runPoll); runJob = "";
    $("#run-cancel").hidden = true;
    const ok = d.exit === 0, secs = Math.round((Date.now() - runStart) / 1000);
    $("#run-state").innerHTML = ok ? `<span class="res ok">✔ başarılı</span> · ${secs} sn` : `<span class="res fail">✖ başarısız</span> · çıkış kodu ${d.exit} · ${secs} sn`;
    // sonuç özeti: motorun "Sonuç:" bölümündeki anahtar=değer satırları
    const all = (txt || "").split("\n");
    const i = all.lastIndexOf("Sonuç:");
    if (i >= 0) {
      const chips = all.slice(i + 1).filter(l => /^\s{2}[A-Za-z0-9_]+=/.test(l)).map(l => `<span class="chip">${esc(l.trim())}</span>`).join("");
      if (chips) $("#run-result").innerHTML = `<div class="small muted" style="margin-bottom:6px">Sonuç</div><div class="chips">${chips}</div>`;
    }
    if (!ok) $("#run-result").insertAdjacentHTML("beforeend", `<div class="notice danger" style="margin-top:10px"><span class="ico" aria-hidden="true">✕</span><div>Eylem başarısız oldu. Ayrıntı için yukarıdaki çıktıya bakın; tam günlük <b>Günlükler</b> sayfasında.</div></div>`);
    $("#run-close").hidden = false; $("#run-close").textContent = "Kapat";
    // kartları yenile (sonuç dosyaları motor tarafından yazıldı)
    if (runMode === "upgrade") { if (view === "packages") { await loadPackages(); checkUpdatesUI(); } load(); return; }
    if (runMode === "task") { if (ok) addDone.add(runSrv + ":" + runAct); await load(); renderAddSteps(); if (view === "server" && srvName) { try { srv = await json("/api/server?name=" + encodeURIComponent(srvName)); renderServer(); } catch (e) { /* sonraki otomatik yenileme alır */ } } return; }
    if (runMode === "remove") {
      if (ok && delPurge) { try { await json("/api/server-purge", { name: delName }); } catch (e) { toast("Günlükler silinemedi: " + e.message); } }
      await load(); if (ok) { srvName = ""; srv = null; showView("servers"); toast(`${delName} silindi`); }
      return;
    }
    try { srv = await json("/api/server?name=" + encodeURIComponent(runSrv)); if (view === "server") renderServer(); load(); } catch (e) { /* bir sonraki turda yenilenir */ }
  }
  $("#run-go").addEventListener("click", startRun);
  $("#run-ext").addEventListener("click", startTerm);
  $("#run-close").addEventListener("click", closeRun);
  $("#run-cancel").addEventListener("click", async () => { $("#run-cancel").disabled = true; try { await json("/api/run/cancel", { id: runJob }); } catch (e) { toast("İptal edilemedi: " + e.message); } });

  /* ---------------- Tema ve Ayarlar ---------------- */
  const THEMES = ["auto", "light", "dark"], THEME_TXT = { auto: "otomatik", light: "açık", dark: "koyu" }, THEME_ICO = { auto: "◐", light: "☀", dark: "☾" };
  let theme = "auto";
  try { theme = THEMES.includes(localStorage.getItem("sshpocket.theme")) ? localStorage.getItem("sshpocket.theme") : "auto"; } catch (e) { /* depolama kapalı olabilir */ }
  function applyTheme() {
    if (theme === "auto") document.documentElement.removeAttribute("data-theme"); else document.documentElement.setAttribute("data-theme", theme);
    const b = $("#theme-btn"); b.setAttribute("aria-label", "Tema: " + THEME_TXT[theme]); b.title = "Tema: " + THEME_TXT[theme] + " (tıklayınca değişir)";
    $("#theme-ico").textContent = THEME_ICO[theme];
    $$("#set-theme button").forEach(x => x.setAttribute("aria-pressed", String(x.dataset.v === theme)));
  }
  function setTheme(v) { theme = v; try { localStorage.setItem("sshpocket.theme", v); } catch (e) { /* önemli değil */ } applyTheme(); }
  $("#theme-btn").addEventListener("click", () => setTheme(THEMES[(THEMES.indexOf(theme) + 1) % THEMES.length]));
  $$("#set-theme button").forEach(b => b.addEventListener("click", () => setTheme(b.dataset.v)));
  $$("#set-mode button").forEach(b => b.addEventListener("click", () => { mode = b.dataset.v; try { localStorage.setItem("sshpocket.mode", mode); } catch (e) { /* önemli değil */ } render(); paintMode(); }));
  const paintMode = () => $$("#set-mode button").forEach(x => x.setAttribute("aria-pressed", String(x.dataset.v === mode)));
  applyTheme();
  const paintAsk = () => $$("#set-ask button").forEach(x => { x.setAttribute("aria-pressed", String(x.dataset.v === askMode)); if (x.dataset.v === "system") { x.disabled = !askAvail; x.title = askAvail ? "" : "Bu platformda sistem penceresi yok"; } });
  $$("#set-ask button").forEach(b => b.addEventListener("click", () => { askMode = b.dataset.v; try { localStorage.setItem("sshpocket.askpass", askMode); } catch (e) { /* önemli değil */ } paintAsk(); }));

  async function loadSettings() {
    paintMode(); applyTheme(); paintAsk();
    let a;
    try { a = await json("/api/about"); askAvail = !!a.askpass; paintAsk(); } catch (e) { $("#set-paths").innerHTML = `<div class="notice danger"><span class="ico" aria-hidden="true">✕</span><div>Yüklenemedi: ${esc(e.message)}</div></div>`; return; }
    const row = (k, v) => `<div class="urow" style="grid-template-columns:minmax(120px,.6fr) 1.6fr"><div class="muted">${esc(k)}</div><div class="mono small" style="word-break:break-all">${esc(v) || "—"}</div></div>`;
    $("#set-paths").innerHTML = [["KeyLab sürümü", a.version], ["Kök klasör (USB)", a.root], ["Sunucular", a.serversDir], ["Paketler", a.componentsDir], ["Durum ve günlükler", a.stateDir], ["Platform", a.platform], ["Go", a.go]].map(([k, v]) => row(k, v)).join("");
    const conf = Object.entries(a.conf || {});
    $("#set-conf").innerHTML = conf.length ? conf.map(([k, v]) => row(k, v)).join("") : '<div class="small muted" style="padding:12px 16px">sshpocket.conf bulunamadı ya da boş.</div>';
  }

  /* ---------------- Sunucu yeniden adlandırma ve silme ---------------- */
  const dlg = $("#dlg");
  let dlgOk = null, delName = "", delPurge = false;
  function openDlg({ title, sub, body, ok, okClass, onOk, wide, noOk }) {
    $("#dlg-title").textContent = title; $("#dlg-sub").textContent = sub || ""; $("#dlg-body").innerHTML = body; $("#dlg-err").textContent = "";
    const b = $("#dlg-ok"); b.textContent = ok; b.className = "btn " + (okClass || "primary"); b.disabled = false; dlgOk = onOk;
    b.hidden = !!noOk; $("#dlg-cancel").textContent = noOk ? "Kapat" : "Vazgeç"; { const m = $(".modal", dlg); m.style.width = wide ? "min(860px,94vw)" : "min(580px,94vw)"; m.style.maxWidth = wide ? "860px" : ""; }
    dlg.classList.add("open");
  }
  const closeDlg = () => dlg.classList.remove("open");
  $("#dlg-x").addEventListener("click", closeDlg); $("#dlg-cancel").addEventListener("click", closeDlg);
  $("#dlg-ok").addEventListener("click", () => dlgOk && dlgOk());
  document.addEventListener("keydown", e => { if (e.key === "Escape") closeDlg(); });

  function openRename(name) {
    openDlg({
      title: "Sunucuyu yeniden adlandır", sub: name,
      body: `<div class="field"><label for="rn-new">Yeni ad</label><input id="rn-new" autocomplete="off" spellcheck="false" autocapitalize="off" maxlength="40" value="${esc(name)}"><div class="hint" id="rn-hint">Küçük harfle başlar; yalnızca küçük harf, rakam ve _ .</div></div>
        <ul class="small muted" style="margin:12px 0 0;padding-left:18px;line-height:1.6"><li>Klasör ve anahtar dosyaları (<span class="mono">id_ed25519_${esc(name)}</span>) yeni ada taşınır; değişkenler ve sunucudaki kayıt geçmişi korunur.</li><li>Günlük ve sonuç geçmişi de yeni ada taşınır.</li><li><b>macOS Keychain</b>'deki passphrase ve Knock kaydı otomatik taşınmaz; passphrase kullanıyorsanız yeniden eklemeniz gerekir.</li></ul>`,
      ok: "Yeniden adlandır",
      onOk: async () => {
        const nn = $("#rn-new").value.trim();
        if (!/^[a-z][a-z0-9_]*$/.test(nn) || nn.length > 40) { $("#dlg-err").textContent = "Geçersiz ad."; return; }
        if (nn === name) { $("#dlg-err").textContent = "Yeni ad eskisiyle aynı."; return; }
        $("#dlg-ok").disabled = true;
        try { await json("/api/server-rename", { old: name, new: nn }); }
        catch (e) { $("#dlg-ok").disabled = false; $("#dlg-err").textContent = "✖ " + e.message; return; }
        closeDlg(); toast(`${name} → ${nn}`); await load(); openServer(nn);
      },
    });
    setTimeout(() => { const i = $("#rn-new"); i.focus(); i.select(); i.addEventListener("keydown", e => { if (e.key === "Enter") $("#dlg-ok").click(); }); }, 50);
  }

  async function openDelete(name) {
    let info;
    try { info = await json("/api/server-files?name=" + encodeURIComponent(name)); }
    catch (e) { toast("Okunamadı: " + e.message); return; }
    const files = info.files || [], hasKey = files.some(f => f.isKey), state = info.state || [];
    openDlg({
      title: "Sunucuyu sil", sub: name,
      body: `<div class="notice danger"><span class="ico" aria-hidden="true">⚠</span><div><b>Bu işlem geri alınamaz.</b> <span class="mono">servers/${esc(name)}/</span> klasörü ve içindeki her şey silinir${hasKey ? "; <b>özel SSH anahtarı dahil</b>" : ""}. Sunucunun kendisine dokunulmaz.</div></div>
        <div class="small muted" style="margin:12px 0 4px">Silinecek dosyalar</div>
        <div class="chips">${files.map(f => `<span class="chip" style="${f.isKey ? "border-color:var(--danger-border);color:var(--danger)" : ""}">${esc(f.name)}</span>`).join("") || '<span class="small muted">boş klasör</span>'}</div>
        ${hasKey ? '<label class="small" style="display:flex;gap:8px;align-items:flex-start;margin-top:14px"><input type="checkbox" id="dl-key"> <span><b>Özel anahtarın silineceğini anlıyorum</b>; sunucuya giriş için tek yol buysa erişimi kaybedebilirim.</span></label>' : ""}
        ${state.length ? `<label class="small" style="display:flex;gap:8px;align-items:flex-start;margin-top:10px"><input type="checkbox" id="dl-logs"> <span>Bu bilgisayardaki günlük ve sonuç geçmişini de sil (${state.map(esc).join(", ")}). İşaretlemezseniz günlükler kalır.</span></label>` : ""}
        <div class="field" style="margin-top:14px"><label for="dl-name">Onay için sunucu adını yazın: <span class="mono">${esc(name)}</span></label><input id="dl-name" autocomplete="off" spellcheck="false" placeholder="${esc(name)}"><div class="hint">Büyük/küçük harf duyarlıdır. Motor Keychain kaydını silmek isteyip istemediğinizi ayrıca sorabilir.</div></div>`,
      ok: "Sil", okClass: "dangerbtn",
      onOk: async () => {
        $("#dlg-ok").disabled = true;
        let r;
        try { r = await json("/api/server-remove", { name, confirm: $("#dl-name").value, ack: !!($("#dl-key") && $("#dl-key").checked) }); }
        catch (e) { $("#dlg-ok").disabled = false; $("#dlg-err").textContent = "✖ " + e.message; return; }
        delName = name; delPurge = !!($("#dl-logs") && $("#dl-logs").checked);
        closeDlg();
        openRun("", "", "remove"); runMode = "remove";
        $("#run-title").innerHTML = `Sunucu siliniyor <span class="draft-tag" style="background:var(--danger-weak);color:var(--danger);border-color:var(--danger-border)">${esc(name)}</span>`;
        $("#run-sub").textContent = "KeyLab motoru (ModuleManager.sh remove)";
        watchJob(r.id);
      },
    });
    const check = () => { const okc = $("#dl-name").value === name && (!$("#dl-key") || $("#dl-key").checked); $("#dlg-ok").disabled = !okc; };
    $("#dlg-ok").disabled = true;
    ["#dl-name", "#dl-key"].forEach(id => { const e = $(id); if (e) { e.addEventListener("input", check); e.addEventListener("change", check); } });
    setTimeout(() => $("#dl-name").focus(), 50);
  }

  /* ---------------- Sunucu ekle ---------------- */
  let addServer = "";
  const addDone = new Set();
  const TASKS = {
    keygen: ["Yeni anahtar üret", "Anahtar oluştur", "ed25519 anahtarı <span class=\"mono\">servers/&lt;ad&gt;/id_ed25519_&lt;ad&gt;</span> olarak oluşturulacak (var olan anahtarın üzerine yazılmaz). Passphrase belirlemeniz istenecek (<b>boş bırakmanız önerilmez</b>): parola penceresi iki kez açılır."],
    keycopy: ["Anahtarı sunucuya gönder", "Anahtarı sunucuya gönder", "Genel anahtar sunucuya eklenecek (<span class=\"mono\">ssh-copy-id</span>). Motor parmak izini gösterip onay ister; sunucunun <b>ilk bağlantı sorusunu</b> (host anahtarı) dikkatle okuyun. Sunucudaki kullanıcının parolası istenirse parola penceresine yazın."],
    keychain: ["Keychain parolası", "Passphrase'i Keychain'e kaydet / güncelle / sil", "Anahtarın passphrase'i macOS Keychain'e (<span class=\"mono\">USBMonitor_&lt;ÖNEK&gt;_Passphrase</span>) kaydedilir; eylemler anahtarı parola sormadan ajana ekleyebilir. Motor önce ne yapacağınızı sorar (1 kaydet · 2 sil · 0 geri); kayıttan sonra anahtar açılarak doğrulanır."],
    knock: ["Port knocking", "Port knocking ayarları", "SSH portu kapalıyken önceden kararlaştırılmış bir port sırasına 'vurarak' açtırma yöntemi (istemci tarafı). Protokol, bekleme ve dizi sorulur; dizi Keychain'e kaydedilir. Sunucuda knock kurulumu yönetici tarafından yapılmış olmalıdır. Bağlanırken ve Terminal'de açarken knock otomatik gönderilir."],
    sshadd: ["ssh-agent'a ekle", "Anahtarı ssh-agent'a ekle", "Anahtar ssh-agent'a eklenecek (macOS'ta Keychain'e kaydedilerek); eylemler parola sormadan bağlanabilsin. Anahtarın passphrase'i istenecek."],
  };
  function openTask(server, task) {
    openRun(server, "", task);
    runMode = "task";
    const [, title, text] = TASKS[task];
    $("#run-title").innerHTML = `${esc(title)} <span class=\"draft-tag\" style=\"background:var(--warn-weak);color:var(--warn);border-color:var(--warn-border)\">${esc(server)}</span>`;
    $("#run-sub").textContent = "Bağlantı hazırlığı";
    $("#run-confirm-text").innerHTML = text;
    $("#run-confirm-note").textContent = "Motorun soruları çıktıda belirir: Evet/Hayır düğmeleriyle ya da kutuya yazarak yanıtlayın. Parolalar sistem parola penceresine yazılır.";
    $("#run-go").textContent = "Başlat";
  }
  function renderAddSteps() {
    const wrap = $("#add-steps");
    if (!addServer) { wrap.hidden = true; $("#add-form-wrap").hidden = false; return; }
    const sv = home.servers.find(x => x.name === addServer);
    $("#add-form-wrap").hidden = true; wrap.hidden = false;
    const hasKey = !!(sv && sv.hasKey);
    const done = t => addDone.has(addServer + ":" + t);
    const step = (n, title, desc, st, btn) => `<div class="act" style="display:grid;grid-template-columns:34px 1fr auto;gap:12px;align-items:center"><div class="badge ${st === "ok" ? "ok" : st === "todo" ? "warn" : "plain"}" style="justify-content:center">${st === "ok" ? "✔" : n}</div><div><div style="font-weight:600">${title}</div><div class="small muted">${desc}</div></div><div>${btn || ""}</div></div>`;
    wrap.innerHTML = `<div class="notice ok" style="margin-bottom:12px"><span class="ico" aria-hidden="true">✔</span><div><b>${esc(addServer)}</b> oluşturuldu. Sunucuyu kullanıma hazırlamak için adımları sırayla izleyin; her adım isteğe bağlıdır ve yeniden yapılabilir.</div></div>
      <div class="card"><div class="card-body tight">
        ${step(1, "Sunucu kaydı", `<span class="mono">servers/${esc(addServer)}/server.env</span> yazıldı`, "ok")}
        ${step(2, "SSH anahtarı", hasKey ? "Anahtar var" : "Yeni bir ed25519 anahtarı üretin (ya da mevcut anahtarı <span class=\"mono\">servers/" + esc(addServer) + "/</span> içine koyun)", hasKey ? "ok" : "todo", hasKey ? "" : '<button class="btn sm primary" data-task="keygen">Yeni anahtar üret</button>')}
        ${step(3, "Anahtarı sunucuya gönder", "Genel anahtarı sunucudaki ~/.ssh/authorized_keys dosyasına ekler (ssh-copy-id)", done("keycopy") ? "ok" : hasKey ? "todo" : "idle", hasKey ? '<button class="btn sm" data-task="keycopy">Gönder…</button>' : "")}
        ${step(4, "ssh-agent'a ekle", "Eylemlerin parola sormadan bağlanması için (macOS Keychain)", done("sshadd") ? "ok" : hasKey ? "todo" : "idle", hasKey ? '<button class="btn sm" data-task="sshadd">Ekle…</button>' : "")}
        ${step(5, "Keychain parolası", "Passphrase'i Keychain'e kaydeder (parolalı anahtar kullanıyorsanız)", done("keychain") ? "ok" : hasKey ? "todo" : "idle", hasKey ? '<button class="btn sm" data-task="keychain">Ayarla…</button>' : "")}
        ${step(6, "Port knocking (isteğe bağlı)", "Sunucu SSH portunu knock dizisiyle açıyorsa", done("knock") ? "ok" : "idle", '<button class="btn sm" data-task="knock">Ayarla…</button>')}
        ${step(7, "Bağlantıyı dene", "Salt okunur <span class=\"mono\">utility durum</span> çalıştırır: ssh bağlantısı, platform ve saat denetlenir", "idle", hasKey ? '<button class="btn sm" id="add-test">Dene</button>' : "")}
        ${step(8, "Değişkenler", "linux-hardening için <span class=\"mono\">hardening_ssh_allow_from</span> gibi zorunlu değerleri ayarlayın", "idle", '<button class="btn sm" id="add-vars">Değişkenleri ayarla</button>')}
      </div></div>
      <div class="row-flex" style="margin-top:14px"><button class="btn primary" id="add-open">Sunucu sayfasına git</button><button class="btn ghost" id="add-another">Yeni bir sunucu daha ekle</button></div>`;
    $$("#add-steps [data-task]").forEach(b => b.addEventListener("click", () => openTask(addServer, b.dataset.task)));
    $("#add-test")?.addEventListener("click", () => openRun(addServer, "utility", "durum"));
    $("#add-vars").addEventListener("click", async () => { await openServer(addServer, "linux-hardening"); openVars(addServer, "linux-hardening"); });
    $("#add-open").addEventListener("click", () => openServer(addServer));
    $("#add-another").addEventListener("click", () => { addServer = ""; $("#add-name").value = $("#add-host").value = $("#add-user").value = $("#add-port").value = ""; renderAddSteps(); schedule(); });
  }
  let addReady = false, addTimer = 0;
  const addTouched = new Set();
  async function initAdd() {
    if (addReady) return;
    addReady = true;
    try {
      const pr = (await json("/api/profiles")) || [];
      $("#add-os").insertAdjacentHTML("beforeend", pr.map(p => `<option value="${esc(p.id)}">${esc(p.id)} — ${esc(Object.entries(p.support).map(([k, v]) => `${TAB_LABEL[k] || k}: ${v === "supported" ? "destekli" : v === "planned" ? "planlı" : v}`).join(", "))}</option>`).join(""));
    } catch (e) { $("#e-os").textContent = "Profiller alınamadı: " + e.message; }
    ["name", "host", "user", "port", "os"].forEach(f => {
      const el = $("#add-" + f);
      el.addEventListener("input", () => { addTouched.add(f); schedule(); });
      el.addEventListener("change", () => { addTouched.add(f); schedule(); });
      el.addEventListener("blur", () => { addTouched.add(f); schedule(); });
    });
    schedule();
  }
  function schedule() { clearTimeout(addTimer); addTimer = setTimeout(checkAdd, 150); }
  async function checkAdd() {
    const body = { name: $("#add-name").value, host: $("#add-host").value, user: $("#add-user").value, port: $("#add-port").value, os: $("#add-os").value };
    let d;
    try { d = await json("/api/server-draft", body); } catch (e) { $("#add-preview").innerHTML = `<div class="notice danger"><span class="ico" aria-hidden="true">✕</span><div>Doğrulanamadı: ${esc(e.message)}</div></div>`; return; }
    ["name", "host", "user", "port", "os"].forEach(f => {
      const msg = d.errors && d.errors[f], show = !!msg && addTouched.has(f);
      const hint = $("#e-" + f);
      hint.closest(".field").classList.toggle("bad", show);
      hint.classList.toggle("err", show);
      hint.textContent = show ? msg : f === "name" ? "Küçük harf, rakam ve _ ; harfle başlar." : f === "os" ? "Paketlerin desteklediği profiller listelenir." : "";
    });
    if (!d.ok) { $("#add-preview").innerHTML = '<div class="empty-state" style="padding:24px 12px"><h3>Önizleme</h3><p>Alanlar geçerli olunca oluşturulacak dosya ve sonraki adımlar burada görünür.</p></div>'; return; }
    $("#add-preview").innerHTML = `
      <div class="small muted">Oluşturulacak dosya <span class="draft-tag">yazılmayacak</span></div>
      <div class="mono small" style="margin-top:6px">${esc(d.envPath)}</div>
      <div class="codeblock">${esc(d.envText)}</div>
      <div class="small muted">Anahtar</div>
      <div class="mono small" style="margin:6px 0 10px">${esc(d.keyPath)}</div>
      <div class="small muted">Sonraki adımlar (terminalde, parolayı ve passphrase'i orada siz girersiniz)</div>
      ${d.commands.map((c, i) => `<div class="codeblock" data-i="${i}">${esc(c)}</div>`).join("")}
      <div class="row-flex"><button class="btn sm" id="add-copy">Komutları kopyala</button><button class="btn sm primary" id="add-create">Sunucuyu oluştur</button></div>
      <div id="add-err" class="small" style="color:var(--danger);margin-top:8px" role="alert"></div>`;
    $("#add-create").addEventListener("click", async () => {
      $("#add-create").disabled = true; $("#add-err").textContent = "";
      try { await json("/api/server-create", { name: $("#add-name").value, host: $("#add-host").value, user: $("#add-user").value, port: $("#add-port").value, os: $("#add-os").value }); }
      catch (e) { $("#add-create").disabled = false; $("#add-err").textContent = "✖ " + e.message; return; }
      addServer = $("#add-name").value.trim(); toast("Sunucu oluşturuldu"); await load(); renderAddSteps();
    });
    $("#add-copy").addEventListener("click", async () => { try { await navigator.clipboard.writeText(d.commands.join("\n")); toast("Komutlar kopyalandı"); } catch { prompt("Kopyalayın:", d.commands.join("\n")); } });
  }

  /* ---------------- Paketler sayfası ---------------- */
  async function loadPackages() {
    let d;
    try { d = await json("/api/packages"); }
    catch (e) { $("#pkg-page").innerHTML = `<div class="notice danger"><span class="ico" aria-hidden="true">✕</span><div>Yüklenemedi: ${esc(e.message)}</div></div>`; return; }
    pkgRepo = Object.fromEntries((d.packages || []).map(p => [p.id, p.repo])); pkgInfo = Object.fromEntries((d.packages || []).map(p => [p.id, p]));
    const cmp = (a, b) => { const x = String(a).split(".").map(Number), y = String(b).split(".").map(Number); for (let i = 0; i < Math.max(x.length, y.length); i++) { const p = x[i] || 0, q = y[i] || 0; if (p !== q) return p < q ? -1 : 1; } return 0; };
    d.packages = d.packages || []; d.servers = d.servers || [];
    if (!d.packages.length) { $("#pkg-page").innerHTML = '<div class="card"><div class="card-body"><div class="empty-state"><h3>Paket bulunamadı</h3><p>USB\'de <span class="mono">components/&lt;paket&gt;/current</span> yok. Paketler menüden ya da <span class="mono">sshpocket</span> ile yüklenir.</p></div></div></div>'; return; }
    const order = ["linux-hardening", "utility", "kurulum"];
    d.packages.sort((a, b) => (order.indexOf(a.id) + 1 || 99) - (order.indexOf(b.id) + 1 || 99));
    $("#pkg-page").innerHTML = `<div id="upd-card"></div><div class="notice" style="margin-bottom:16px"><span class="ico" aria-hidden="true">ℹ</span><div>Bir pakete tıklayın: sürümler, kilit tabanı ve sunuculardaki son kullanım açılır. "Kullanıldı", paketin o sunucuda en son çalıştırıldığı anlamına gelir; paketler sunucuya kurulmaz, kontrolcüden çalıştırılır.</div></div>` +
      `<div class="card pk-list">` + (d.packages.filter(p => !query || `${p.id} ${p.name} ${p.repo || ""}`.toLowerCase().includes(query)).map(p => {
        const sm = pkgSummary(p, d.servers, cmp);
        const floorBad = p.floor && cmp(p.current, p.floor) < 0;
        return `<button class="pk-li" data-pk="${esc(p.id)}" aria-label="${esc(p.name || p.id)} ayrıntıları">
          <span class="pk-li-ico" aria-hidden="true">${esc((p.name || p.id).charAt(0).toUpperCase())}</span>
          <span class="pk-li-main"><span class="pk-li-name">${esc(p.name || p.id)}</span><span class="pk-li-sub mono">${esc(p.id)}${p.repo ? " · " + esc(p.repo) : ""}</span></span>
          <span class="pk-li-ver"><span class="vchip cur" title="USB'de kullanılan (current)">${esc(p.current)}</span><span class="small muted">taban ${p.floor ? esc(p.floor) : "—"}</span></span>
          <span class="pk-li-use small">${sm.used ? `${sm.used} sunucuda kullanıldı` : '<span class="muted">hiçbir sunucuda kullanılmadı</span>'}</span>
          <span class="pk-li-badges">${floorBad ? '<span class="badge danger">kilit tabanının altında</span>' : ""}${sm.stale ? `<span class="badge warn">${sm.stale} sunucuda eski sürüm</span>` : ""}<span data-upd="${esc(p.id)}"></span></span>
          <span class="pk-li-go" aria-hidden="true">›</span></button>`;
      }).join("") || '<div class="card-body small muted">Paket bulunamadı.</div>') + "</div>";
    $$("#pkg-page [data-pk]").forEach(b => b.addEventListener("click", () => openPkgDetail(d.packages.find(p => p.id === b.dataset.pk), d.servers, cmp)));
    renderUpdates();
  }

  /* paket → sunucu özeti (liste satırı ve ayrıntı penceresi aynı sayıları kullanır) */
  function pkgSummary(p, servers, cmp) {
    let used = 0, stale = 0;
    servers.forEach(s => { const u = (p.uses || []).find(x => x.server === s.name); if (u) { used++; if (cmp(u.version, p.current) < 0) stale++; } });
    return { used, stale, none: servers.length - used };
  }

  /* paket ayrıntı penceresi: sürümler, kilit, sunucularda son kullanım (çok sunucuda süzülebilir) */
  function openPkgDetail(p, servers, cmp) {
    if (!p) return;
    const sm = pkgSummary(p, servers, cmp), many = servers.length > 8;
    const floorBad = p.floor && cmp(p.current, p.floor) < 0;
    const box = (l, v) => `<div class="spec-box"><span class="spec-label">${l}</span><div class="spec-value">${v}</div></div>`;
    const rowsHtml = (onlyStale, showNone, q) => {
      const rs = servers.filter(s => !q || s.name.toLowerCase().includes(q)).map(s => {
        const u = (p.uses || []).find(x => x.server === s.name);
        if (!u && !showNone) return "";
        const c = u ? cmp(u.version, p.current) : 0;
        if (onlyStale && !(u && c < 0)) return "";
        const ver = !u ? '<span class="muted">kullanılmadı</span>' : c === 0 ? '<span class="badge ok">USB ile aynı</span>' : c < 0 ? '<span class="badge warn">USB\'de daha yeni var</span>' : '<span class="badge info">USB\'den yeni</span>';
        const note = u ? `${u.result === "ok" ? '<span class="res ok">✔</span>' : '<span class="res fail">✖</span>'} ${esc(u.action)} · <span class="muted" title="${esc(localTime(u.time))}">${esc(ago(u.time))} önce</span>` : "";
        return `<div class="urow"><div class="who">${osIcon(s.os, 26)}${esc(s.name)}</div><div>${u ? `<span class="mono small">${esc(u.version)}</span>` : "—"}</div><div>${ver}</div><div class="small">${note}</div></div>`;
      }).join("");
      return rs || '<div class="urow"><div class="muted small">Gösterilecek sunucu yok.</div></div>';
    };
    openDlg({
      title: p.name || p.id, sub: `${p.id}${p.repo ? " · " + p.repo : ""}`, wide: true, noOk: true,
      body: `<p class="small" style="margin:0;color:var(--text-2)">${esc(p.description || "")}</p>
        <div class="spec-grid">
          ${box("USB sürüm havuzu", (p.versions || []).map(v => `<span class="vchip${v === p.current ? " cur" : ""}">${esc(v)}</span>`).join("") || "—")}
          ${box("Kilit tabanı", `${p.floor ? `<span class="vchip">${esc(p.floor)}</span>` : '<span class="muted small">kilitte yok</span>'}${floorBad ? '<span class="badge danger">USB sürümü tabanın altında</span>' : p.floor ? '<span class="small muted" style="font-weight:400">bu sürümün altı çalıştırılmaz</span>' : ""}`)}
          ${box("Sunucularda", `${sm.used} kullandı${sm.stale ? ` · <span style="color:var(--warn)">${sm.stale} eski</span>` : ""}${sm.none ? ` · <span class="muted" style="font-weight:400">${sm.none} kullanmadı</span>` : ""}`)}
        </div>
        <div class="pk-fleet-head"><b>Sunucularda son kullanım</b><span class="small muted">paket sunucuya kurulmaz; bu, en son çalıştırıldığı sürümdür</span></div>
        ${many || sm.stale ? `<div class="row-flex" style="gap:12px;flex-wrap:wrap">
          ${many ? '<input id="pd-q" type="search" placeholder="sunucu ara…" aria-label="Sunucu ara" style="max-width:220px">' : ""}
          <label class="small" style="display:flex;gap:6px;align-items:center"><input type="checkbox" id="pd-stale"${many && sm.stale ? " checked" : ""}> yalnız eski sürümlüler</label>
          ${sm.none ? `<label class="small" style="display:flex;gap:6px;align-items:center"><input type="checkbox" id="pd-none"${many ? "" : " checked"}> kullanılmayanları da göster (${sm.none})</label>` : ""}</div>` : ""}
        <div class="pk-fleet"><div class="urow head"><div>Sunucu</div><div>Son sürüm</div><div>Durum</div><div>Son eylem</div></div><div id="pd-rows"></div></div>`,
      ok: "",
    });
    const paint = () => {
      const q = ($("#pd-q")?.value || "").trim().toLowerCase();
      $("#pd-rows").innerHTML = rowsHtml($("#pd-stale")?.checked || false, $("#pd-none") ? $("#pd-none").checked : !many, q);
    };
    ["#pd-q", "#pd-stale", "#pd-none"].forEach(id => $(id)?.addEventListener(id === "#pd-q" ? "input" : "change", paint));
    paint();
  }

  /* imzalı paket indeksi: motorun `--components update` raporu (ağdan alınır, imzası motor tarafından doğrulanır) */
  let updRep = null, updAt = 0, updBusy = false, updErr = "", pkgRepo = {}, pkgInfo = {}, discQ = "", discF = "all";
  const VERDICT = { upgrade: ["warn", "⬆ yükseltilebilir"], install: ["info", "📥 kurulabilir"], current: ["ok", "✔ güncel"], yanked: ["danger", "🚫 önerilmiyor (yanked)"], "needs-sshpocket": ["warn", "⏸ sshpocket güncellemesi gerekli"], "other-repo": ["danger", "başka depo gösteriyor"], "below-floor": ["danger", "kilit tabanının altında"], conflict: ["danger", "çelişki"], unknown: ["plain", "bilinmiyor"] };
  /* Sayfada tek satırlık özet; ayrıntı ve eylemler modalda */
  const updUp = () => updRep && updRep.ok ? updRep.rows.filter(r => r.verdict === "upgrade" || r.verdict === "install").length : 0;
  const isUpdDlg = () => dlg.classList.contains("open") && $("#dlg-title").textContent === "Güncellemeler";
  function renderUpdates() {
    const el = $("#upd-card"); if (!el) return;
    let st;
    if (updBusy) st = '<span class="spinner" aria-hidden="true"></span> İmzalı indeks alınıyor ve doğrulanıyor…';
    else if (updErr) st = '<span style="color:var(--danger)">✕ denetlenemedi</span>';
    else if (!updRep) st = '<span class="muted">henüz denetlenmedi · hiçbir şey kurmaz</span>';
    else if (!updRep.ok) st = '<span style="color:var(--danger)">✕ indeks doğrulanamadı (güvenli kapanma)</span>';
    else st = updUp() ? `<b style="color:var(--warn)">⬆ ${updUp()} güncelleme var</b>` : '<span style="color:var(--ok)">✔ hepsi güncel</span>';
    el.innerHTML = `<div class="card"><div class="card-head"><h2>Güncellemeler</h2><span class="hint">imzalı indeks · syhgorath/sshpocket-index</span><span style="flex:1"></span><span class="small">${st}</span><button class="btn sm" id="disc-open">Paketleri keşfet</button><button class="btn sm primary" id="upd-check"${updBusy ? " disabled" : ""}>${updRep || updErr ? "Ayrıntı" : "Güncellemeleri denetle"}</button></div></div>`;
    $("#upd-check").addEventListener("click", openUpdates);
    $("#disc-open").addEventListener("click", openDiscover);
    $$("#pkg-page [data-upd]").forEach(sp => { const r = updRep && updRep.ok ? updRep.rows.find(x => x.id === sp.dataset.upd) : null; sp.innerHTML = r && (r.verdict === "upgrade" || r.verdict === "install") ? `<span class="badge info">⬆ ${esc(r.version)} çıktı</span>` : ""; });
    if (isUpdDlg()) paintUpdDlg(); else if (isDiscDlg()) paintDisc();
  }
  function updBodyHtml() {
    const head = `<div class="row-flex" style="gap:10px;flex-wrap:wrap"><span class="small muted">Motorun <span class="mono">--components update</span> raporu: imza ve süre doğrulanır, hiçbir şey kurulmaz.</span><span style="flex:1"></span><button class="btn sm" id="upd-recheck"${updBusy ? " disabled" : ""}>Yeniden denetle</button></div>`;
    if (updBusy) return head + '<div class="small muted"><span class="spinner" aria-hidden="true"></span> İmzalı indeks alınıyor ve doğrulanıyor…</div>';
    if (updErr) return head + `<div class="notice danger"><span class="ico" aria-hidden="true">✕</span><div>${esc(updErr)}</div></div>`;
    if (!updRep) return head;
    if (!updRep.ok) return head + `<div class="notice danger"><span class="ico" aria-hidden="true">✕</span><div><b>İndeks doğrulanamadı</b> (güvenli kapanma): yükseltme yapılmaz.</div></div><details><summary class="small muted" style="cursor:pointer">motor çıktısı</summary><pre class="mono small" style="white-space:pre-wrap">${esc(updRep.raw)}</pre></details>`;
    const rows = updRep.rows.map(r => { const [c, t] = VERDICT[r.verdict] || VERDICT.unknown; return `<div class="pkg-row"><div><div class="pkg-name">${esc(r.id)}</div><div class="pkg-meta"><span class="ver">${esc(r.installed === "-" ? "kurulu değil" : r.installed)}</span>${r.version ? `<span class="ver arrow">${esc(r.version)}</span>` : ""}<span class="badge ${c}">${t}</span></div></div>
      <div class="pkg-actions">${r.verdict === "upgrade" || r.verdict === "install" ? `<button class="btn sm primary" data-upg="${esc(r.id)}" data-from="${esc(r.installed)}" data-to="${esc(r.version)}">${r.verdict === "install" ? "Kur" : "Yükselt"}…</button>` : ""}</div></div>`; }).join("");
    return head + `<div class="small muted">İndeks seri <span class="mono">${esc(updRep.serial)}</span> · geçerlilik <span class="mono">${esc(updRep.valid)}</span> · ${updUp() ? `<b style="color:var(--warn)">${updUp()} güncelleme var</b>` : "hepsi güncel"}${updAt ? ` · denetim ${new Date(updAt).toLocaleTimeString("tr-TR")}` : ""}</div>
      <div>${rows}</div><details><summary class="small muted" style="cursor:pointer">motorun ham raporu</summary><pre class="mono small" style="white-space:pre-wrap;margin:8px 0 0">${esc(updRep.raw)}</pre></details>`;
  }
  function paintUpdDlg() {
    $("#dlg-body").innerHTML = updBodyHtml();
    $("#upd-recheck")?.addEventListener("click", checkUpdatesUI);
    $$("#dlg-body [data-upg]").forEach(b => b.addEventListener("click", () => { const { upg, from, to } = b.dataset; closeDlg(); openUpgrade(upg, from, to); }));
  }
  function openUpdates() {
    openDlg({ title: "Güncellemeler", sub: "imzalı indeks · syhgorath/sshpocket-index", body: "", wide: true, noOk: true, ok: "" });
    paintUpdDlg();
    if (!updRep && !updErr && !updBusy) checkUpdatesUI();
  }

  /* Paketleri keşfet: imzalı indeksteki paketler (apt deposu gibi göz atma); kurma/yükseltme mevcut onay akışından geçer */
  const isDiscDlg = () => dlg.classList.contains("open") && $("#dlg-title").textContent === "Paketleri keşfet";
  const DISC_F = [["all", "Tümü"], ["installed", "USB'de var"], ["install", "Kurulabilir"], ["upgrade", "Güncellenebilir"]];
  function discRows() {
    if (!updRep || !updRep.ok) return [];
    return updRep.rows.filter(r => {
      const have = r.installed !== "-";
      if (discF === "installed" && !have) return false;
      if (discF === "install" && r.verdict !== "install") return false;
      if (discF === "upgrade" && r.verdict !== "upgrade") return false;
      const inf = pkgInfo[r.id] || {};
      return !discQ || `${r.id} ${inf.name || ""} ${inf.description || ""}`.toLowerCase().includes(discQ);
    });
  }
  function paintDiscList() {
    const rs = discRows();
    $("#disc-list").innerHTML = rs.map(r => {
      const inf = pkgInfo[r.id] || {}, [c, t] = VERDICT[r.verdict] || VERDICT.unknown, have = r.installed !== "-";
      const act = r.verdict === "upgrade" || r.verdict === "install";
      return `<div class="disc-row"><span class="pk-li-ico" aria-hidden="true">${esc((inf.name || r.id).charAt(0).toUpperCase())}</span>
        <div class="disc-main"><div class="pkg-name">${esc(inf.name || r.id)}${inf.name && inf.name !== r.id ? ` <span class="small muted mono">${esc(r.id)}</span>` : ""}</div>
          <div class="small muted">${inf.description ? esc(inf.description) : have ? "" : "Açıklama, paket USB'ye indirildikten sonra görünür."}</div>
          <div class="pkg-meta"><span class="ver">${have ? esc(r.installed) : "kurulu değil"}</span>${r.version ? `<span class="ver arrow">${esc(r.version)}</span>` : ""}<span class="badge ${c}">${t}</span></div></div>
        ${act ? `<button class="btn sm primary" data-dget="${esc(r.id)}" data-from="${esc(r.installed)}" data-to="${esc(r.version)}">${r.verdict === "install" ? "Kur" : "Yükselt"}…</button>` : ""}</div>`;
    }).join("") || '<div class="small muted" style="padding:12px 0">Bu süzgeçle eşleşen paket yok.</div>';
    $$("#disc-list [data-dget]").forEach(b => b.addEventListener("click", () => { const { dget, from, to } = b.dataset; closeDlg(); openUpgrade(dget, from, to); }));
  }
  function paintDisc() {
    if (updBusy) { $("#dlg-body").innerHTML = '<div class="small muted"><span class="spinner" aria-hidden="true"></span> İmzalı indeks alınıyor ve doğrulanıyor…</div>'; return; }
    if (updErr || !updRep || !updRep.ok) { $("#dlg-body").innerHTML = `<div class="notice danger"><span class="ico" aria-hidden="true">✕</span><div>${updErr ? esc(updErr) : "İndeks doğrulanamadı (güvenli kapanma): paket listesi gösterilmez."}</div></div><div><button class="btn sm" id="disc-retry">Yeniden dene</button></div>`; $("#disc-retry").addEventListener("click", checkUpdatesUI); return; }
    const cnt = k => updRep.rows.filter(r => k === "all" || (k === "installed" ? r.installed !== "-" : r.verdict === k)).length;
    $("#dlg-body").innerHTML = `<div class="row-flex" style="gap:10px;flex-wrap:wrap"><input id="disc-q" type="search" placeholder="paket ara…" aria-label="Paket ara" value="${esc(discQ)}" style="max-width:240px"><div class="seg" role="tablist" aria-label="Süzgeç">${DISC_F.map(([k, l]) => `<button class="btn sm${discF === k ? " primary" : ""}" data-df="${k}">${l} (${cnt(k)})</button>`).join("")}</div></div>
      <div class="small muted">Kaynak: imzalı indeks (seri <span class="mono">${esc(updRep.serial)}</span>). Kur ve Yükselt, imza ve özet doğrulamasıyla USB'ye yazar; sunucuya dokunmaz.</div>
      <div id="disc-list"></div>`;
    $("#disc-q").addEventListener("input", e => { discQ = e.target.value.trim().toLowerCase(); paintDiscList(); });
    $$("#dlg-body [data-df]").forEach(b => b.addEventListener("click", () => { discF = b.dataset.df; paintDisc(); }));
    paintDiscList();
  }
  function openDiscover() {
    openDlg({ title: "Paketleri keşfet", sub: "imzalı indeks · syhgorath/sshpocket-index", body: "", wide: true, noOk: true, ok: "" });
    paintDisc();
    if (!updRep && !updErr && !updBusy) checkUpdatesUI();
  }
  async function checkUpdatesUI() {
    if (updBusy) return;
    updBusy = true; updErr = ""; renderUpdates();
    try { updRep = await json("/api/updates"); updAt = Date.now(); }
    catch (e) { updErr = "Denetlenemedi: " + e.message; }
    updBusy = false; renderUpdates();
  }

  let runUpgTo = "";
  function openUpgrade(pkg, from, to) {
    openRun("", pkg, "upgrade");
    runMode = "upgrade"; runUpgTo = to;
    const ov = pkgRepo[pkg];
    $("#run-title").innerHTML = `<span class="mono">upgrade</span> · ${esc(pkg)} <span class="draft-tag" style="background:var(--warn-weak);color:var(--warn);border-color:var(--warn-border)">USB'ye yazar</span>`;
    $("#run-sub").textContent = "KeyLab paket güncellemesi (imzalı indeks)";
    $("#run-confirm-text").innerHTML = `<b>${esc(pkg)}</b> ${from === "-" ? "kurulacak" : `<span class="mono">${esc(from)}</span> → <span class="mono">${esc(to)}</span> yükseltilecek`}: ${from === "-" ? `<span class="mono">${esc(to)}</span> sürümü ` : ""}indirilip <b>imzası ve özeti doğrulanarak</b> USB'deki <span class="mono">components/</span> klasörüne yazılacak. Eski sürüm klasörü silinmez; hiçbir sunucuya dokunulmaz.`;
    $("#run-confirm-extra").hidden = false;
    $("#run-confirm-extra").innerHTML = `<label class="small" style="display:flex;gap:8px;align-items:flex-start;margin-top:12px"><input type="checkbox" id="u-ack"> <span>İndeksin gösterdiği kaynaktan${ov ? ` (<span class="mono">${esc(ov)}</span>)` : ""} indirilen sürümü kurmak istiyorum.</span></label>`;
    $("#run-confirm-note").textContent = "Çalıştır'a bastığınızda indeks yeniden alınıp imzası doğrulanır; gösterilen sürüm değişmişse kurulum durur. İmza, özet, depo eşleşmesi ve sürüm kapıları kurulum katmanının işidir; biri tutmazsa kurulum durur ve eski sürüm olduğu gibi kalır. Kurulum atomiktir, başladıktan sonra iptal edilemez.";
    $("#run-go").textContent = "Çalıştır"; $("#run-go").disabled = true;
    $("#u-ack").addEventListener("change", e => { $("#run-go").disabled = !e.target.checked; });
  }

  /* ---------------- Lynis sayfası ---------------- */
  let lynSel = "", lynData = null, lynKind = "all", lynQ = "", lynPre = ""; // lynPre: sayfa açılırken seçilecek tür süzgeci

  function chart(l) {
    const h = l.history || [], d = l.dates || [], W = Math.max(300, Math.min(1000, (($(".main") || {}).clientWidth || 700) - 70)), H = 190, L = 30, R = 10, T = 10, B = 22;
    if (h.length < 1) return "";
    const x = i => (h.length === 1 ? (L + W - R) / 2 : L + (i / (h.length - 1)) * (W - L - R));
    const y = v => T + (1 - Math.max(0, Math.min(100, v)) / 100) * (H - T - B);
    const pts = h.map((v, i) => [Math.round(x(i)), Math.round(y(v))]);
    const grid = [0, 25, 50, 75, 100].map(g => `<line class="gl" x1="${L}" x2="${W - R}" y1="${y(g)}" y2="${y(g)}"/><text class="axis" x="${L - 6}" y="${y(g) + 3}" text-anchor="end">${g}</text>`).join("");
    const line = pts.map(p => p.join(",")).join(" ");
    const area = h.length > 1 ? `<path class="ar" d="M${pts.map(p => p.join(",")).join(" L")} L${pts[pts.length - 1][0]},${H - B} L${pts[0][0]},${H - B} Z"/>` : "";
    const dots = pts.map((p, i) => `<circle class="pt${i === pts.length - 1 ? " last" : ""}" cx="${p[0]}" cy="${p[1]}" r="${i === pts.length - 1 ? 4 : 3}"><title>${esc((d[i] || "").slice(0, 16).replace("T", " "))} · skor ${h[i]}</title></circle>`).join("");
    const lab = (i, anchor) => `<text class="axis" x="${pts[i][0]}" y="${H - 6}" text-anchor="${anchor}">${esc((d[i] || "").slice(5, 10))}</text>`;
    return `<svg viewBox="0 0 ${W} ${H}" role="img" aria-label="Lynis skoru geçmişi: ${h.join(", ")}">${grid}${area}${h.length > 1 ? `<polyline class="ln" points="${line}"/>` : ""}${dots}${lab(0, "start")}${h.length > 1 ? lab(h.length - 1, "end") : ""}</svg>`;
  }

  function findingRow(f) {
    const kind = f.kind === "uyari" ? '<span class="badge warn">uyarı</span>' : '<span class="badge info">öneri</span>';
    const fix = f.value
      ? `<div class="fchips"><span class="chip">${esc(f.value)}</span>${f.closesByDefault === "evet" ? '<span class="badge ok">paket uygulayınca kapanır</span>' : f.closesByDefault === "hayir" ? '<span class="badge plain">isteğe bağlı (varsayılan kapalı)</span>' : ""}${f.area ? `<span class="badge plain">alan: ${esc(f.area)}</span>` : ""}</div>`
      : "";
    const sub = [f.detail, f.suggestion].filter(Boolean).map(esc).join(" · ");
    return `<div class="frow"><div class="fid">${esc(f.id)}</div><div>${kind}</div><div><div class="ftext">${esc(f.text)}</div>${sub ? `<div class="fsub">${sub}</div>` : ""}${fix}</div></div>`;
  }

  function paintFindings() {
    if (!lynData) return;
    const q = lynQ.toLowerCase();
    const list = lynData.findings.filter(f => (lynKind === "all" || f.kind === lynKind) && (!q || `${f.id} ${f.text} ${f.value} ${f.area}`.toLowerCase().includes(q)));
    $("#lyn-count").textContent = `${list.length} / ${lynData.findings.length} bulgu`;
    $("#lyn-rows").innerHTML = list.map(findingRow).join("") || '<div class="frow"><div></div><div></div><div class="muted">Eşleşen bulgu yok.</div></div>';
  }

  function paintLynis() {
    const d = lynData, l = d && d.lynis;
    if (!l) { $("#lynis-detail").innerHTML = `<div class="card"><div class="card-body small muted">Bu sunucu için ölçüm yok. Menüden linux-hardening <span class="mono">audit</span> çalıştırın.</div></div>`; return; }
    const first = l.history[0], delta = l.score - first;
    $("#lynis-detail").innerHTML = `
      <div class="card"><div class="card-head"><h2>Skor geçmişi — ${esc(d.server)}</h2><div class="spacer"></div><span class="hint">${l.history.length} ölçüm · rapor ${esc(d.report)}</span></div>
        <div class="card-body"><div class="lynis-head"><div class="lynis-score">${l.score}</div><div class="small muted" style="align-self:flex-end;margin-bottom:4px">/100</div>
          ${l.history.length > 1 ? `<div class="lynis-delta">${delta >= 0 ? "▲ +" : "▼ "}${delta}</div>` : ""}</div>
          <div class="lynis-meta"><span class="badge warn">uyarı ${l.warnings}</span><span class="badge info">öneri ${l.suggestions}</span>${l.history.length > 1 ? `<span class="badge plain">${first} → ${l.score}</span>` : ""}</div>
          <div class="chartbox">${chart(l)}</div>
          <div class="small muted" style="margin-top:6px">Geçmiş, bu kontrolcüde kayıtlı tüm ölçümleri içerir; VM snapshot dönüşlerinden önceki ölçümler de dahildir.</div></div></div>
      <div class="card"><div class="card-head"><h2>Bulgular</h2><div class="spacer"></div>
        <div class="segmented" role="group" aria-label="Tür"><button data-k="all" aria-pressed="${lynKind === "all"}">Tümü</button><button data-k="uyari" aria-pressed="${lynKind === "uyari"}">Uyarı</button><button data-k="oneri" aria-pressed="${lynKind === "oneri"}">Öneri</button></div>
        <input id="lyn-q" type="search" value="${esc(lynQ)}" placeholder="ara…" aria-label="Bulgularda ara" style="max-width:170px;margin-left:8px"><span class="hint" id="lyn-count" style="margin-left:8px"></span></div>
        <div class="card-body tight"><div class="fgrid" id="lyn-rows"></div></div></div>`;
    $$("#lynis-detail .segmented button").forEach(b => b.addEventListener("click", () => { lynKind = b.dataset.k; $$("#lynis-detail .segmented button").forEach(x => x.setAttribute("aria-pressed", String(x === b))); paintFindings(); }));
    $("#lyn-q").addEventListener("input", e => { lynQ = e.target.value.trim(); paintFindings(); });
    paintFindings();
  }

  async function loadLynis(name) {
    if (!home.servers.length) { $("#lynis-pick").innerHTML = ""; $("#lynis-detail").innerHTML = '<div class="card"><div class="card-body"><div class="empty-state"><h3>Sunucu yok</h3><p>Lynis ölçümü için önce bir sunucu eklenmeli.</p></div></div></div>'; return; }
    const have = home.servers.filter(s => s.lynis).sort((a, b) => (b.lynis.date || "").localeCompare(a.lynis.date || ""));
    if (name) lynSel = name;
    if (!lynSel || !home.servers.some(s => s.name === lynSel)) lynSel = (have[0] || home.servers[0] || {}).name || selected || "";
    $("#lynis-pick").innerHTML = home.servers.map(s => `<button class="pickbtn" role="tab" aria-selected="${s.name === lynSel}" data-n="${esc(s.name)}">${osIcon(s.os, 26)}<span>${esc(s.name)}${s.lynis ? ` <b>${s.lynis.score}</b>` : ' <span class="muted">ölçüm yok</span>'}</span></button>`).join("");
    $$("#lynis-pick .pickbtn").forEach(b => b.addEventListener("click", () => loadLynis(b.dataset.n)));
    if (!lynSel) { $("#lynis-detail").innerHTML = ""; return; }
    lynKind = lynPre || "all"; lynPre = ""; lynQ = "";
    try { lynData = await json("/api/lynis?server=" + encodeURIComponent(lynSel)); }
    catch (e) { $("#lynis-detail").innerHTML = `<div class="notice danger"><span class="ico" aria-hidden="true">✕</span><div>Yüklenemedi: ${esc(e.message)}</div></div>`; return; }
    paintLynis();
  }

  /* ---------------- Günlükler ---------------- */
  let curLog = null;
  let logAll = [], logSrv = "", logPkg = "", logKind = "all", logQ = "", logKey = "";
  const logId = l => `${l.package}/${l.server}/${l.file}`;
  const isLast = l => l.file === "last-run.log";

  async function loadLogs() {
    try { logAll = (await json("/api/logs")) || []; }
    catch (e) { toast("Günlükler alınamadı: " + e.message); return; }
    paintLogs();
  }

  function paintLogs() {
    if (!logAll.length) { $("#logfilters").innerHTML = ""; $("#logs-count").textContent = "0 dosya"; $("#loglist").innerHTML = '<div class="empty-state"><h3>Henüz günlük yok</h3><p>Bir paket eylemi çalıştırılınca günlükleri <span class="mono">~/.local/share/sshpocket/&lt;paket&gt;/&lt;sunucu&gt;/</span> altında burada görünür.</p></div>'; return; }
    // süzgeç düğmeleri: değerler ve sayıları günlüklerden çıkar
    const count = (key) => logAll.reduce((m, l) => (m[l[key]] = (m[l[key]] || 0) + 1, m), {});
    const srvs = count("server"), pkgs = count("package");
    const chip = (val, label, cur, attr, n) => `<button class="fchip" data-${attr}="${esc(val)}" aria-pressed="${cur === val}">${label}${n !== undefined ? ` <span class="muted">${n}</span>` : ""}</button>`;
    const byServer = Object.keys(srvs).sort().map(k => chip(k, esc(k), logSrv, "srv", srvs[k])).join("");
    const byPkg = Object.keys(pkgs).sort().map(k => chip(k, esc(TAB_LABEL[k] || k), logPkg, "pkg", pkgs[k])).join("");
    const filtered = logAll.filter(l =>
      (!logSrv || l.server === logSrv) && (!logPkg || l.package === logPkg) &&
      (logKind === "all" || (logKind === "last" ? isLast(l) : !isLast(l))) &&
      (!logQ || `${l.server} ${l.package} ${l.file}`.toLowerCase().includes(logQ)));
    $("#logfilters").innerHTML = `
      <div class="frow2"><span class="flabel">Sunucu</span><div class="fchips2">${chip("", "Tümü", logSrv, "srv")}${byServer}</div></div>
      <div class="frow2"><span class="flabel">Paket</span><div class="fchips2">${chip("", "Tümü", logPkg, "pkg")}${byPkg}</div></div>
      <div class="frow2"><span class="flabel">Tür</span><div class="fchips2">
        <button class="fchip" data-kind="all" aria-pressed="${logKind === "all"}">Tümü</button>
        <button class="fchip" data-kind="last" aria-pressed="${logKind === "last"}">Son çalıştırma</button>
        <button class="fchip" data-kind="arch" aria-pressed="${logKind === "arch"}">Arşiv / kopya</button>
        <input id="logq" type="search" placeholder="dosya adında ara…" aria-label="Günlük dosyalarında ara" value="${esc(logQ)}" style="max-width:190px;margin-left:8px">
        ${(logSrv || logPkg || logKind !== "all" || logQ) ? '<button class="btn sm ghost" id="logclear">Süzgeçleri temizle</button>' : ""}</div></div>`;
    $("#logs-count").textContent = `${filtered.length} / ${logAll.length} dosya`;
    $("#loglist").innerHTML = filtered.map((l, i) => `<button data-i="${i}" aria-current="${logId(l) === logKey}"><span class="mono">${osIconSmall(l.server)}${esc(l.server)} · ${esc(l.package)} · ${esc(l.file)}</span>
      <span class="muted small" title="${esc(localTime(l.mtime))}">${esc(ago(l.mtime))} · ${Math.max(1, Math.round(l.size / 1024))} KB</span></button>`).join("") || '<div class="small muted" style="padding:12px">Süzgece uyan günlük yok.</div>';
    $$("#loglist button[data-i]").forEach(b => b.addEventListener("click", () => openLog(filtered[+b.dataset.i], b)));
    $$("#logfilters [data-srv]").forEach(b => b.addEventListener("click", () => { logSrv = b.dataset.srv; paintLogs(); }));
    $$("#logfilters [data-pkg]").forEach(b => b.addEventListener("click", () => { logPkg = b.dataset.pkg; paintLogs(); }));
    $$("#logfilters [data-kind]").forEach(b => b.addEventListener("click", () => { logKind = b.dataset.kind; paintLogs(); }));
    const q = $("#logq"); q.addEventListener("input", () => { logQ = q.value.trim().toLowerCase(); const pos = q.selectionStart; paintLogs(); const n = $("#logq"); n.focus(); n.setSelectionRange(pos, pos); });
    const c = $("#logclear"); if (c) c.addEventListener("click", () => { logSrv = logPkg = logQ = ""; logKind = "all"; paintLogs(); });
  }
  // sunucu adına göre küçük simge (liste satırı için)
  const osIconSmall = name => { const s = home.servers.find(x => x.name === name); return s ? osIcon(s.os, 16) + " " : ""; };

  async function openLog(l, btn) {
    logKey = logId(l);
    $$("#loglist button").forEach(b => b.removeAttribute("aria-current"));
    if (btn) btn.setAttribute("aria-current", "true");
    try {
      const q = new URLSearchParams({ package: l.package, server: l.server, file: l.file });
      curLog = await (await api("/api/log?" + q)).text();
    } catch (e) { toast("Açılamadı: " + e.message); return; }
    $("#logcard").hidden = false;
    $("#logtitle").textContent = `${l.server} · ${l.package} · ${l.file}`;
    $("#logfind").value = "";
    paintLog();
    $("#logcard").scrollIntoView({ block: "nearest" });
  }
  function paintLog(term) {
    const el = $("#logview"); el.textContent = "";
    const lines = (curLog || "").split("\n");
    const re = term ? new RegExp(term.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"), "gi") : null;
    const frag = document.createDocumentFragment();
    lines.forEach((ln, i) => {
      const d = document.createElement("div"); d.dataset.n = i;
      if (/fatal:|FAILED|failed=[1-9]|ERROR|HATA/.test(ln)) d.className = "err";
      if (re && re.test(ln)) {
        re.lastIndex = 0; d.textContent = "";
        let last = 0, m;
        while ((m = re.exec(ln))) { d.append(ln.slice(last, m.index)); const k = document.createElement("mark"); k.textContent = m[0]; d.append(k); last = m.index + m[0].length; if (!m[0]) break; }
        d.append(ln.slice(last));
      } else d.textContent = ln || " ";
      frag.appendChild(d);
    });
    el.appendChild(frag);
  }
  $("#logfind").addEventListener("input", e => { paintLog(e.target.value); const m = $("#logview mark"); if (m) m.scrollIntoView({ block: "center" }); });
  $("#logerr").addEventListener("click", () => { const e = $("#logview .err"); if (e) { e.scrollIntoView({ block: "center" }); e.classList.add("search-hit"); setTimeout(() => e.classList.remove("search-hit"), 1500); } else toast("Hata satırı yok"); });

  /* ---------------- Sunucu sayfası ---------------- */
  const TAB_LABEL = { "linux-hardening": "Hardening", utility: "Utility", kurulum: "Kurulum" };
  const TAB_ORDER = ["linux-hardening", "utility", "kurulum"];
  let srv = null;      // /api/server yanıtı
  let srvName = "", srvTab = "";

  const effectBadges = a => {
    const e = a.effects || [], out = [];
    if (!e.length) out.push('<span class="badge ok">salt okunur</span>');
    if (e.includes("installs-package")) out.push('<span class="badge warn">paket kurar</span>');
    if (e.includes("changes-config")) out.push('<span class="badge warn">yapılandırmayı değiştirir</span>');
    if (e.includes("destructive")) out.push('<span class="badge danger">yıkıcı</span>');
    if (a.requiresSudo) out.push('<span class="badge plain">sudo gerekir</span>');
    if (a.confirm === "yes-no") out.push('<span class="badge plain">e/h onayı</span>');
    if (a.confirm === "type-server-name") out.push('<span class="badge info">sunucu adıyla onay</span>');
    return out.join("");
  };

  function actionCard(a, pkg) {
    const byId = id => pkg.actions.find(x => x.id === id);
    const req = (a.prereqs || []).map(q => {
      if (q.ok) return `<span class="ok">✔ ${esc(q.action)}</span> <span class="muted">${Math.max(1, Math.round(q.ageSec / 60))} dk önce · ${Math.max(0, 30 - Math.round(q.ageSec / 60))} dk daha geçerli</span>`;
      if (q.result === "ok") return `<span class="warn">⏱ ${esc(q.action)} eski</span> <span class="muted">(30 dk'yı aştı; yeniden çalıştırın)</span>`;
      if (q.result) return `<span class="danger">✖ ${esc(q.action)} başarısız</span>`;
      return `<span class="warn">${esc(q.action)} henüz çalışmadı</span>`;
    });
    const keys = a.keys ? Object.entries(a.keys).map(([k, v]) => `<span class="chip">${esc(k)}=${esc(v)}</span>`).join("") : "";
    const changed = (a.effects || []).length > 0;
    const newer = !changed && a.lastTime && pkg.actions.some(x => (x.effects || []).length > 0 && x.lastTime && Date.parse(x.lastTime) > Date.parse(a.lastTime));
    const stale = newer ? '<span class="badge warn stale" title="Bu sonuçtan sonra bu sunucuda değişiklik yapan bir eylem çalıştı; değerler güncel olmayabilir. Eylemi yeniden çalıştırınca yenilenir.">güncel olmayabilir</span>' : "";
    const last = a.lastResult
      ? `${a.lastResult === "ok" ? '<span class="res ok">✔ başarılı</span>' : '<span class="res fail">✖ başarısız</span>'} <span class="muted" title="${esc(localTime(a.lastTime))}">${esc(ago(a.lastTime))} önce</span>`
      : (a.lastTime ? `<span class="muted" title="${esc(localTime(a.lastTime))}">${esc(ago(a.lastTime))} önce</span>` : '<span class="muted">bu sunucuda henüz çalışmadı</span>');
    const w = a.warnings || [];
    const warn = w.length ? `<ul>${w.slice(0, 2).map(x => `<li>${esc(x)}</li>`).join("")}</ul>` +
      (w.length > 2 ? `<details style="margin-top:4px"><summary class="small muted" style="cursor:pointer">${w.length - 2} uyarı daha</summary><ul>${w.slice(2).map(x => `<li>${esc(x)}</li>`).join("")}</ul></details>` : "") : "";
    const cat = actCat(a);
    return `<div class="act r-${cat}" data-cat="${cat}"><div class="act-head"><span class="act-name">${esc(a.id)}</span>${effectBadges(a)}${stale}</div>
      ${warn}${keys ? `<div class="chips">${keys}</div>` : ""}
      <div class="act-foot"><span>${last}</span>${req.length ? `<span>· önkoşul: ${req.join(", ")}</span>` : ""}<span class="grow"></span>
        ${a.runnable ? `<button class="btn sm primary" data-run="${esc(pkg.id)}|${esc(a.id)}">Çalıştır</button>` : a.write ? (a.destructive ? `<button class="btn sm dangerbtn" data-write="${esc(pkg.id)}|${esc(a.id)}" title="YIKICI: geri alınamayabilir; plan, snapshot, ad yazma ve ek onaydan geçer">Yıkıcı eylem…</button>` : `<button class="btn sm warnbtn" data-write="${esc(pkg.id)}|${esc(a.id)}" title="Gerçek sunucuya yazar; plan, snapshot ve onay adımlarından geçer">Onayla ve çalıştır…</button>`) : a.terminal ? `<button class="btn sm" data-term="${esc(pkg.id)}|${esc(a.id)}" title="Sudo parolası gerekir; Terminal'de girersiniz">Terminal'de çalıştır</button>` : `<span class="small muted" style="max-width:300px;text-align:right">${esc(a.reason || "")}</span>`}</div></div>`;
  }

  // paketin bu sunucudaki değişkenleri (salt okunur): ayarlı olanlar ve eylemlerin zorunlu kıldıkları
  function varsCard(p) {
    const vs = p.vars || [];
    const editBtn = `<span style="flex:1"></span><button class="btn sm" data-vars="${esc(p.id)}">Düzenle…</button>`;
    if (!vs.length) return `<div class="card"><div class="card-head"><h2>Değişkenler</h2><span class="hint">bu sunucuda ayarlı değişken yok</span>${editBtn}</div><div class="card-body small muted">Hiç değişken ayarlanmamış; paket varsayılanlarıyla çalışır. <b>Düzenle…</b> ile tüm değişkenleri görüp ayarlayabilirsiniz.</div></div>`;
    const row = v => {
      const shown = v.set ? v.value : v.default;
      const state = v.set ? '<span class="badge ok">ayarlı</span>' : v.needed ? '<span class="badge danger">eksik · gerekli</span>' : '<span class="badge plain">varsayılan</span>';
      const full = v.description || "", desc = full.length > 150 ? full.slice(0, 150).replace(/\s+\S*$/, "") + "…" : full;
      return `<div class="urow" style="grid-template-columns:minmax(170px,1fr) minmax(140px,1fr) 2fr"><div><div class="mono small" style="font-weight:600">${esc(v.name)}</div><div>${state}${v.needed && v.set ? ' <span class="badge plain">gerekli</span>' : ""}</div></div>
        <div><span class="chip">${esc(shown) || "(boş)"}</span></div><div class="small muted" title="${esc(full)}">${esc(desc)}</div></div>`;
    };
    return `<div class="card"><div class="card-head"><h2>Değişkenler</h2><span class="hint">bu sunucu için · ${vs.length} ayarlı/gerekli</span>${editBtn}</div>
      <div class="card-body tight">${vs.map(row).join("")}</div>
      <div class="card-body small muted" style="border-top:1px solid var(--border)">Eylemler bu değişkenlerle çalışır. Tüm değişkenleri görmek ve değiştirmek için <b>Düzenle…</b>.</div></div>`;
  }

  /* ---------------- Sunucu özeti ---------------- */
  const COPY_ICON = '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15V6a2 2 0 0 1 2-2h9"/></svg>';
  const GAUGE_PATH = "M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831";
  const gauge = score => { const v = Math.max(0, Math.min(100, score)), c = v >= 80 ? "ok" : v >= 60 ? "warn" : "danger"; return `<div class="gauge ${c}"><svg viewBox="0 0 36 36" aria-hidden="true"><path class="g-bg" stroke-width="3" fill="none" d="${GAUGE_PATH}"/><path class="g-fg" stroke-dasharray="${v}, 100" stroke-width="3.2" stroke-linecap="round" fill="none" d="${GAUGE_PATH}"/></svg><div class="g-num"><b>${v}</b><span>/100</span></div></div>`; };

  // Bağlantı hazırlığı satırı: st = ok | warn | todo | idle (ok dışında adım numarası görünür)
  const prepRow = (n, st, title, badge, desc, btn) => `<div class="prep-row"><div class="prep-dot ${st}" aria-hidden="true">${st === "ok" ? "✓" : st === "warn" ? "!" : n}</div><div class="prep-txt"><div class="prep-title">${title}${badge ? ` <span class="prep-badge ${st}">${badge}</span>` : ""}</div><div class="small muted">${desc}</div></div>${btn || ""}</div>`;

  function renderOzet(s, pk) {
    const k = (srv && srv.knock) || {}, pr = (srv && srv.prep) || {};
    const knockOn = !!k.enabled;
    // adım durumları (yalnızca bu bilgisayarda bilinenler; "sunucuya gönderildi mi" bağlanmadan bilinemez)
    const steps = [
      ["keygen", s.hasKey ? "ok" : "todo"],
      ["sshadd", !s.hasKey ? "idle" : pr.agent ? "ok" : "todo"],
      ["keychain", !s.hasKey ? "idle" : pr.passphrase ? "ok" : "todo"],
      ...(knockOn ? [["knock", k.hasSeq ? "ok" : "warn"]] : []),
    ];
    const done = steps.filter(x => x[1] === "ok").length;
    const btn = (task, label) => `<button class="btn sm" data-task="${task}" ${s.valid ? "" : "disabled"}>${label}</button>`;
    const prep = [
      prepRow(1, s.hasKey ? "ok" : "todo", "SSH anahtarı", s.hasKey ? "var" : "", s.hasKey ? "Anahtar bu sunucunun klasöründe mevcut." : "Yeni bir ed25519 anahtarı üretir (var olan anahtarın üzerine yazılmaz).", s.hasKey ? "" : btn("keygen", "Anahtar üret")),
      prepRow(2, "idle", "Anahtarı sunucuya gönder", "", 'Genel anahtarı sunucudaki <span class="mono">authorized_keys</span> dosyasına ekler (ssh-copy-id). Sunucuda olup olmadığı bağlanmadan bilinemez.', s.hasKey ? btn("keycopy", "Gönder…") : ""),
      prepRow(3, steps[1][1], "ssh-agent'a ekle", pr.agent ? "ajanda" : "", "Eylemlerin parola sormadan bağlanması için anahtarı ajana yükler (macOS Keychain).", s.hasKey ? btn("sshadd", pr.agent ? "Yeniden ekle…" : "Ekle…") : ""),
      prepRow(4, steps[2][1], "Keychain parolası", pr.passphrase ? "bu Mac'te kayıtlı" : "", "Anahtarın passphrase'ini Keychain'e kaydeder, günceller ya da siler (parolasız anahtarda gerekmez).", s.hasKey ? btn("keychain", "Ayarla…") : ""),
      prepRow(5, knockOn ? (k.hasSeq ? "ok" : "warn") : "idle", "Port knocking", knockOn ? (k.hasSeq ? esc(k.hint || "dizi kayıtlı") : "bu Mac'te dizi yok") : "", knockOn ? (k.hasSeq ? `Açık · ${esc(k.proto)}, ${esc(k.delay)} ms. Bağlanmadan önce otomatik gönderilir.` : "Knock açık ama dizi bu bilgisayarın Keychain'inde yok; yeniden girin.") : "Sunucu SSH portunu knock dizisiyle açıyorsa (isteğe bağlı).", btn("knock", knockOn ? "Değiştir…" : "Ayarla…")),
    ].join("");
    const test = pk.some(p => p.id === "utility") ? `<div class="prep-test"><div class="prep-dot test" aria-hidden="true">⚡</div><div class="prep-txt"><div class="prep-title">Bağlantıyı dene</div><div class="small muted">Salt okunur <span class="mono">utility durum</span> çalıştırır: ssh bağlantısı, platform ve saat denetlenir.</div></div><button class="btn primary" data-test ${s.valid && s.hasKey ? "" : "disabled"}>Dene ve doğrula</button></div>` : "";

    const tcp = !s.valid ? ["warn", "yapılandırma geçersiz"] : !s.checked ? ["plain", "denetleniyor"] : s.up ? ["ok", "TCP erişilebilir"] : s.knock ? ["warn", "🔒 kapalı · knock gerekir"] : ["danger", "erişilemiyor"];
    const kvr = (a, b) => `<div class="kv"><span class="muted">${a}</span><span>${b}</span></div>`;
    const l = s.lynis;
    const pkgRows = pk.map(p => `<div class="mini-row"><div class="mini-txt"><div class="mini-title" title="${esc(p.name || p.id)}">${esc(p.name || p.id)}</div><div class="small muted mono">v${esc(p.version)}</div></div><button class="btn sm" data-goto="${esc(p.id)}">Aç</button></div>`).join("") || '<div class="small muted">Bu sunucu için paket yok.</div>';
    const rec = (s.recent || []).slice(0, 4).map(a => `<div class="feed-row ${isOk(a.result) ? "ok" : "fail"}"><div class="feed-top small muted"><span class="mono">${esc(a.package)} ${esc(a.version)}</span><span title="${esc(localTime(a.time))}">${esc(ago(a.time))} önce</span></div><div class="feed-msg">${isOk(a.result) ? "✓" : "✖"} ${esc(a.action)}</div></div>`).join("") || '<div class="small muted">Henüz işlem yok.</div>';

    $("#srv-body").innerHTML = `<div class="ozet4">
      <div class="card oz-card"><div class="oz-top"><span class="oz-title">Durum &amp; ağ</span><span class="badge ${tcp[0]}">${tcp[1]}</span></div>
        <div class="oz-body">${kvr("Sistem", esc((OS_LABEL[s.os] || "Linux") + (s.platform ? " · " + s.platform : "")))}${kvr("SSH anahtarı", s.hasKey ? '<span class="badge ok">var</span>' : '<span class="badge warn">yok</span>')}${kvr("Port knocking", !k.enabled ? '<span class="muted">kapalı</span>' : k.hasSeq ? `açık · ${esc(k.proto)}` : '<span style="color:var(--warn)">açık · dizi yok</span>')}</div>
        <div class="oz-foot">Durum TCP bağlantı denemesiyle ölçülür; sunucuya giriş yapılmaz.</div></div>
      <div class="card oz-card"><div class="oz-top"><span class="oz-title">Sertleştirme</span><span class="oz-tag">Lynis</span></div>
        <div class="oz-body">${l ? `<div class="gauge-row">${gauge(l.score)}<div class="gauge-info"><div class="small muted">100 üzerinden</div><div class="gauge-badges"><span class="badge ${l.warnings ? "warn" : "ok"}">${l.warnings} uyarı</span><span class="badge info">${l.suggestions} öneri</span></div></div></div>` : '<div class="small muted">Bu sunucu için ölçüm yok. linux-hardening <span class="mono">audit</span> eylemi çalıştırılınca skor burada görünür.</div>'}</div>
        <div class="oz-foot oz-foot-row"><span>${l ? "Ölçüm: " + esc((l.date || "").slice(0, 10)) + (auditStale(s) ? ' <span class="stale-flag" title="Son ölçümden sonra sertleştirme uygulandı; yeniden denetleyin">⚠ güncel değil</span>' : "") : ""}</span>${l ? '<button class="link" data-go-lynis>Raporu gör →</button>' : '<button class="link" data-goto="linux-hardening">Hardening →</button>'}</div></div>
      <div class="card oz-card"><div class="oz-top"><span class="oz-title">Paketler</span><span class="oz-tag">bu USB'de</span></div><div class="oz-body">${pkgRows}</div></div>
      <div class="card oz-card"><div class="oz-top"><span class="oz-title">Son işlemler</span></div><div class="oz-body">${rec}</div>
        <div class="oz-foot oz-foot-row"><span></span><button class="link" data-go-logs>Bu sunucunun günlükleri →</button></div></div>
    </div>
    <div class="card prep-card"><div class="prep-head"><div><h2>Bağlantı hazırlığı</h2><div class="small muted">Her adım isteğe bağlıdır ve yeniden yapılabilir.</div></div><span class="prep-count" title="Anahtar, ajan, Keychain parolası${knockOn ? " ve knock" : ""} sayılır; ssh-copy-id sayılmaz">${done}/${steps.length} adım hazır</span></div>
      <div class="prep-list">${prep}${test}</div></div>`;

    $$("#srv-body [data-goto]").forEach(b => b.addEventListener("click", () => { srvTab = b.dataset.goto; history.replaceState(null, "", `#s=${encodeURIComponent(srvName)}&p=${encodeURIComponent(srvTab)}`); renderServer(); }));
    $("#srv-body [data-go-logs]")?.addEventListener("click", () => goLogs(srvName));
    $("#srv-body [data-go-lynis]")?.addEventListener("click", () => goLynis(srvName, s.lynis && s.lynis.warnings > 0 ? "uyari" : "all"));
    $$("#srv-body [data-task]").forEach(b => b.addEventListener("click", () => (b.dataset.task === "knock" ? openKnock(srvName) : openTask(srvName, b.dataset.task))));
    $("#srv-body [data-test]")?.addEventListener("click", () => openRun(srvName, "utility", "durum"));
  }

  /* ---------------- Süzgeçli geçişler ve bağlantı bilgisi ---------------- */
  // Günlükler sayfasını yalnızca bu sunucunun günlükleri süzülmüş olarak aç.
  function goLogs(server) { logSrv = server; logPkg = ""; logKind = "all"; logQ = ""; showView("logs"); }
  // Lynis sayfasını bu sunucu seçili ve (verilmişse) bulgu türü süzülmüş olarak aç.
  function goLynis(server, kind) { lynSel = server; lynPre = kind || ""; showView("lynis"); }

  async function openEditAddr(name) {
    let cur;
    try { cur = (await json("/api/server?name=" + encodeURIComponent(name))).server; }
    catch (e) { toast("Okunamadı: " + e.message); return; }
    openDlg({
      title: "Bağlantı bilgilerini düzenle", sub: name,
      body: `<p class="small muted" style="margin:0 0 12px">Adres, kullanıcı ya da port değişince SSH anahtarı, Keychain parolası ve knock ayarları <b>aynen kalır</b> (sunucu adına bağlıdır). Adres değiştiyse sunucunun parmak izi ilk bağlantıda yeniden sorulabilir.</p>
        <div class="field"><label for="ea-host">Adres (IP ya da sunucu adı)</label><input id="ea-host" autocomplete="off" spellcheck="false" autocapitalize="off" value="${esc(cur.host || "")}"></div>
        <div class="row-flex" style="gap:12px;align-items:flex-start;flex-wrap:wrap">
          <div class="field" style="flex:1;min-width:160px"><label for="ea-user">Kullanıcı</label><input id="ea-user" autocomplete="off" spellcheck="false" autocapitalize="off" value="${esc(cur.user || "")}"></div>
          <div class="field"><label for="ea-port">Port</label><input id="ea-port" type="number" min="1" max="65535" inputmode="numeric" style="width:110px" value="${esc(cur.port || "22")}"></div></div>`,
      ok: "Kaydet",
      onOk: async () => {
        const host = $("#ea-host").value.trim(), user = $("#ea-user").value.trim(), port = $("#ea-port").value.trim() || "22";
        if (host === (cur.host || "") && user === (cur.user || "") && port === (cur.port || "22")) { $("#dlg-err").textContent = "Değişiklik yok."; return; }
        $("#dlg-ok").disabled = true;
        try { await json("/api/server-edit", { name, host, user, port }); }
        catch (e) { $("#dlg-ok").disabled = false; $("#dlg-err").textContent = "✖ " + e.message; return; }
        closeDlg(); toast("Bağlantı bilgileri güncellendi"); await load(); await reloadServer();
      },
    });
    ["#ea-host", "#ea-user", "#ea-port"].forEach(id => $(id).addEventListener("keydown", e => { if (e.key === "Enter") $("#dlg-ok").click(); }));
    setTimeout(() => { const i = $("#ea-host"); i.focus(); i.select(); }, 50);
  }

  /* ---------------- Port knocking formu ---------------- */
  // Dizi Keychain'dedir (bu bilgisayara özgü; USB başka bir Mac'e takılırsa orada yeniden girilmelidir). Arayüz diziyi ASLA geri okumaz.
  function knockText(k) {
    if (!k || !k.enabled) return '<span class="muted">kapalı</span>';
    if (!k.hasSeq) return '<span style="color:var(--warn)">açık, ama dizi bu Mac\'te kayıtlı değil</span>';
    return `açık · ${esc(k.proto)}, ${esc(k.delay)} ms · dizi bu Mac'te kayıtlı`;
  }
  const knockSeqOK = v => /^[0-9]+(,[0-9]+){0,9}$/.test(v) && v.split(",").every(p => +p >= 1 && +p <= 65535);
  async function reloadServer() {
    if (!srvName) return;
    try { srv = await json("/api/server?name=" + encodeURIComponent(srvName)); if (view === "server") renderServer(); } catch (e) { /* sonraki otomatik yenileme alır */ }
  }
  async function openKnock(name) {
    let k;
    try { k = await json("/api/knock?name=" + encodeURIComponent(name)); }
    catch (e) { toast("Okunamadı: " + e.message); return; }
    const state = !k.enabled ? "" : k.hasSeq
      ? `<div class="notice ok" style="margin-bottom:12px"><span class="ico" aria-hidden="true">✔</span><div>Knock <b>açık</b>: ${esc(k.proto)}, ${esc(k.delay)} ms. Dizi bu bilgisayarın Keychain'inde kayıtlı; güvenlik için tamamı gösterilmez${k.hint ? `, yalnızca her portun ilk ve son rakamı: <span class="mono" style="font-weight:600">${esc(k.hint)}</span>` : ""}. Yeni bir dizi girerseniz eskisinin yerine geçer.</div></div>`
      : `<div class="notice warn" style="margin-bottom:12px"><span class="ico" aria-hidden="true">⚠</span><div>Bu sunucuda knock <b>açık</b> görünüyor ama dizi <b>bu bilgisayarın</b> Keychain'inde yok. Dizi USB'ye değil, yazıldığı Mac'e kaydedilir; USB'yi başka bir Mac'e taktıysanız diziyi burada yeniden girin.</div></div>`;
    openDlg({
      title: "Port knocking", sub: name,
      body: `${state}<p class="small muted" style="margin:0 0 12px">SSH portu kapalıyken, sunucu yöneticisinin belirlediği port sırasına "vurarak" açtırma yöntemi. Sunucuda knock kurulumu yönetici tarafından yapılmış olmalıdır.</p>
        <div class="field"><label for="kn-seq">Dizi (port sırası)</label><input id="kn-seq" type="password" autocomplete="off" spellcheck="false" data-lpignore="true" data-1p-ignore="true" placeholder="${k.hasSeq ? "•••••• kayıtlı dizi korunur" : "ör. 7000,8000,9000"}" inputmode="numeric"><div class="hint" id="kn-hint">${k.hasSeq ? "Değiştirmek için yeni diziyi tam olarak yazın. Boş bırakırsanız kayıtlı dizi korunur ve yalnızca protokol ile bekleme güncellenir." : "Sunucu yöneticisinin verdiği PORT SIRASI: virgülle, boşluksuz, en çok 10 port. Adım numarasını değil, portları yazın."}</div>
</div>
        <div class="small" style="margin-top:6px"><label style="display:inline-flex;gap:8px;align-items:center;cursor:pointer"><input type="checkbox" id="kn-show"> Yazdıklarımı göster</label></div>
        <div class="row-flex" style="gap:12px;align-items:flex-end;flex-wrap:wrap;margin-top:12px">
          <div class="field" style="margin:0"><label for="kn-proto">Protokol</label><select id="kn-proto" style="min-width:96px;height:36px"><option value="tcp">tcp</option><option value="udp">udp</option></select></div>
          <div class="field" style="margin:0"><label for="kn-delay">Vuruşlar arası bekleme (ms)</label><input id="kn-delay" type="number" min="0" max="5000" step="50" style="width:110px"></div></div>
        ${k.enabled ? `<div class="row-flex" style="gap:8px;margin-top:16px;flex-wrap:wrap">${k.hasSeq ? '<button class="btn sm" id="kn-send" type="button">Knock\'u şimdi gönder</button>' : ""}<button class="btn sm dangerbtn" id="kn-off" type="button">Knock\'u kapat ve kaydı sil</button></div>` : ""}`,
      ok: "Kaydet",
      onOk: async () => {
        const seq = $("#kn-seq").value.trim(), delay = $("#kn-delay").value.trim();
        if (seq ? !knockSeqOK(seq) : !k.hasSeq) { $("#dlg-err").textContent = seq ? "Dizi 1-10 port olmalı: virgülle ayrılmış, her biri 1-65535, boşluksuz (örnek: 7000,8000,9000)." : "Dizi gerekli: bu bilgisayarda kayıtlı dizi yok (örnek: 7000,8000,9000)."; return; }
        if (!/^[0-9]+$/.test(delay) || +delay > 5000) { $("#dlg-err").textContent = "Bekleme 0 ile 5000 arasında bir sayı olmalı."; return; }
        $("#dlg-ok").disabled = true;
        try { await json("/api/knock", { name, action: "save", proto: $("#kn-proto").value, delay, seq }); }
        catch (e) { $("#dlg-ok").disabled = false; $("#dlg-err").textContent = "✖ " + e.message; return; }
        $("#kn-seq").value = "";
        closeDlg(); toast("Knock kaydedildi"); await reloadServer();
      },
    });
    $("#kn-proto").value = k.proto || "tcp"; $("#kn-delay").value = k.delay || "300";
    const seq = $("#kn-seq"), hint = $("#kn-hint");
    seq.addEventListener("input", () => { const v = seq.value.trim(); hint.classList.toggle("err", !!v && !knockSeqOK(v)); $("#dlg-err").textContent = ""; });
    seq.addEventListener("keydown", e => { if (e.key === "Enter") $("#dlg-ok").click(); });
    $("#kn-show").addEventListener("change", e => { seq.type = e.target.checked ? "text" : "password"; });
    $("#kn-send")?.addEventListener("click", async () => {
      try { await json("/api/knock", { name, action: "send" }); toast("Knock gönderildi"); } catch (e) { $("#dlg-err").textContent = "✖ " + e.message; }
    });
    let sure = false;
    $("#kn-off")?.addEventListener("click", async e => {
      if (!sure) { sure = true; e.target.textContent = "Emin misiniz? Tekrar tıklayın"; return; }
      try { await json("/api/knock", { name, action: "disable" }); } catch (er) { $("#dlg-err").textContent = "✖ " + er.message; return; }
      closeDlg(); toast("Knock kapatıldı"); await reloadServer();
    });
    setTimeout(() => seq.focus(), 50);
  }

  /* ---------------- Paket sayfası (Hardening / Utility / Kurulum) ---------------- */
  let actFilter = "all";
  // eylem kategorisi manifestteki etkilerden türetilir: yıkıcı > değiştirir > salt okunur
  const actCat = a => ((a.effects || []).includes("destructive") || a.destructive ? "danger" : (a.effects || []).length > 0 ? "change" : "read");
  const ACT_CATS = [["all", "Tümü"], ["read", "Salt okunur"], ["change", "Değiştirir"], ["danger", "Yıkıcı"]];
  // "Yeniden denetle" gibi kısayol düğmeleri, eylem kartındaki düğmeyle aynı işleyiciyi kullanır
  function actBtnAttr(a, pkg) { const id = `${esc(pkg.id)}|${esc(a.id)}`; return a.runnable ? `data-run="${id}"` : a.write ? `data-write="${id}"` : a.terminal ? `data-term="${id}"` : ""; }

  // Son başarılı `apply`, son Lynis ölçümünden SONRA ise ölçüm eskidir (skor/bulgular yeni durumu yansıtmaz).
  // Yalnız son 8 işlem bilinir (components.log); daha eski bir apply ancak sonraki bir audit ile birlikte düşer.
  function auditStale(s) {
    const l = s.lynis; if (!l || !l.date) return null;
    const t0 = Date.parse(l.date); if (isNaN(t0)) return null;
    return (s.recent || []).find(a => a.package === "linux-hardening" && a.action === "apply" && isOk(a.result) && Date.parse(a.time) > t0 + 60000) || null;
  }
  function lynisHero(s, p) {
    const l = s.lynis, au = p.actions.find(a => a.id === "audit"), attr = au ? actBtnAttr(au, p) : "";
    const btn = attr ? `<button class="btn" ${attr}>${l ? "Yeniden denetle" : "Denetim çalıştır"}</button>` : "";
    if (!l) return `<div class="card hero-card"><div class="hero-top"><div><div class="hero-kicker">Lynis skoru — ${esc(s.name)}</div><div class="small muted" style="margin-top:6px">Bu sunucu için ölçüm yok. Denetim çalıştırılınca skor ve geçmiş burada görünür.</div></div>${btn}</div></div>`;
    const st = auditStale(s);
    const h = l.history || [], prev = h.length > 1 ? h[h.length - 2] : null, d = prev === null ? 0 : l.score - prev;
    const prevTxt = prev === null ? "henüz yok" : `${prev}${d === 0 ? " (değişmedi)" : ` (${d > 0 ? "▲ +" : "▼ "}${d})`}`;
    return `<div class="card hero-card"><div class="hero-top">
        <div><div class="hero-kicker">Lynis skoru — ${esc(s.name)} <span class="hero-sep" aria-hidden="true"></span> <span class="muted">Son denetim: ${esc((l.date || "").slice(0, 10))}</span>${st ? ' <span class="stale-flag">⚠ güncel değil</span>' : ""}</div>
          <div class="hero-score"><b>${l.score}</b><span class="hero-of">/ 100</span><span class="badge ${l.warnings ? "warn" : "ok"}">${l.warnings} uyarı</span><span class="badge info">${l.suggestions} öneri</span></div></div>
        <div class="hero-side"><div class="hero-meta"><b>Toplam ${h.length} ölçüm</b><span class="muted">Önceki: ${prevTxt}</span></div>${btn}</div></div>
      ${st ? `<div class="notice" style="margin:12px 0 0"><span class="ico" aria-hidden="true">⚠</span><div>Son denetimden sonra sertleştirme uygulandı (<span title="${esc(localTime(st.time))}">${esc(ago(st.time))} önce</span>). Skor ve bulgular eski olabilir; <b>Yeniden denetle</b> ile güncelleyin.</div></div>` : ""}
      <div class="chartbox hero-chart">${chart(l)}</div>
      <div class="oz-foot oz-foot-row"><span>Geçmiş, bu kontrolcüde kayıtlı tüm ölçümleri içerir.</span><button class="link" data-go-lynis-p>Bulguları gör →</button></div></div>`;
  }

  // "Son durum" kartı: paketin salt okunur `durum` eyleminin SON çalıştırma çıktısı (a.keys) kutulara dökülür.
  // Canlı bağlantı yoktur; veri son çalıştırmadan gelir, yaşı ve "güncel olmayabilir" uyarısı gösterilir.
  const ST_LABEL = { locale: "Sistem dili (LANG)", timezone: "Saat dilimi", ntp_senkron: "NTP senkron", ntp_servis: "NTP servisi", kaynak_sayisi: "NTP kaynağı", offset_ms: "Zaman ofseti", sapma_ms: "Sapma", secili_kaynak: "Seçili kaynak", saat_sapma_kontrolcuye_gore_sn: "Kontrol makinesine göre sapma" };
  const ST_MAIN = ["locale", "timezone", "ntp_senkron", "kaynak_sayisi", "offset_ms"];
  function statusCard(p) {
    const a = p.actions.find(x => x.id === "durum");
    if (!a) return "";
    const attr = actBtnAttr(a, p), k = a.keys || {}, has = Object.keys(k).length > 0;
    const stale = a.lastTime && p.actions.some(x => (x.effects || []).length > 0 && x.lastTime && Date.parse(x.lastTime) > Date.parse(a.lastTime));
    const btn = attr ? `<button class="link" ${attr}>${has ? "Şimdi yenile" : "Durumu oku"}</button>` : "";
    const head = `<div class="oz-top"><span class="oz-title">Son durum</span><span class="st-meta">${a.lastTime ? `Son kontrol: ${esc(ago(a.lastTime))} önce` : "henüz okunmadı"}${a.lastResult === "fail" ? ' <span class="badge danger">son çalıştırma başarısız</span>' : ""}${stale ? ' <span class="badge warn" title="Bu okumadan sonra bu sunucuda değişiklik yapan bir eylem çalıştı.">güncel olmayabilir</span>' : ""}${btn ? " · " + btn : ""}</span></div>`;
    if (!has) return `<div class="card st-card">${head}<div class="small muted">Bu sunucuda <span class="mono">durum</span> eylemi henüz çalıştırılmadı; sonucu burada kutular olarak görünür.</div></div>`;
    const unk = v => v === undefined || v === "" || v === "unknown";
    const val = (key, fallback = "—") => (unk(k[key]) ? `<span class="muted">${fallback}</span>` : esc(k[key]));
    const tiles = {
      locale: ["Sistem dili (LANG)", val("locale"), "", ""],
      timezone: ["Saat dilimi", val("timezone"), "", ""],
      ntp_senkron: ["NTP senkron", k.ntp_senkron === "yes" ? "Senkronize" : k.ntp_senkron === "no" ? "Senkron değil" : val("ntp_senkron"), `Servis: ${unk(k.ntp_servis) ? "bilinmiyor" : esc(k.ntp_servis)}`, k.ntp_senkron === "yes" ? "ok" : k.ntp_senkron === "no" ? "warn" : ""],
      kaynak_sayisi: ["NTP kaynağı", unk(k.kaynak_sayisi) ? val("kaynak_sayisi") : `${esc(k.kaynak_sayisi)} kaynak`, k.secili_kaynak === "yes" ? "✓ seçili kaynak var" : k.secili_kaynak === "no" ? "seçili kaynak yok" : "", ""],
      offset_ms: ["Zaman ofseti", unk(k.offset_ms) ? val("offset_ms") : `${esc(k.offset_ms)} ms`, unk(k.sapma_ms) ? "" : `Sapma: ${esc(k.sapma_ms)} ms`, unk(k.sapma_ms) || isNaN(+k.sapma_ms) ? "" : Math.abs(+k.sapma_ms) <= 1000 ? "ok" : "warn"],
    };
    const shown = ST_MAIN.filter(m => m in k);
    const rest = Object.entries(k).filter(([n]) => !shown.includes(n));
    const tile = key => { const [label, v, sub, st] = tiles[key]; return `<div class="st-tile"><div class="st-label">${label}</div><div class="st-val ${st}">${v}</div>${sub ? `<div class="st-sub">${sub}</div>` : ""}</div>`; };
    return `<div class="card st-card">${head}
      ${shown.length ? `<div class="st-grid">${shown.map(tile).join("")}</div>` : ""}
      ${rest.length ? `<details class="st-more"><summary class="small muted">${shown.length ? "Diğer değerler" : "Değerler"} (${rest.length})</summary><div class="chips">${rest.map(([n, v]) => `<span class="chip" title="${esc(n)}">${esc(ST_LABEL[n] || n)}: ${esc(v)}</span>`).join("")}</div></details>` : ""}</div>`;
  }

  function pkgInfoCard(p, ok) {
    const n = c => ok.filter(a => actCat(a) === c).length;
    return `<div class="card pk-info"><div class="oz-top"><span class="oz-title">${esc(p.name || p.id)}</span><span class="oz-tag">${esc(p.id)} ${esc(p.version)} · USB</span></div>
      <p class="small" style="margin:0 0 12px;color:var(--text-2);line-height:1.55">${esc(p.description)}</p>
      <div class="pk-counts"><span class="badge plain">${ok.length} eylem</span>${n("read") ? `<span class="badge ok">${n("read")} salt okunur</span>` : ""}${n("change") ? `<span class="badge warn">${n("change")} değiştirir</span>` : ""}${n("danger") ? `<span class="badge danger">${n("danger")} yıkıcı</span>` : ""}</div></div>`;
  }

  function actionsSection(ok, na, p) {
    const chips = ACT_CATS.map(([k, label]) => { const n = k === "all" ? ok.length : ok.filter(a => actCat(a) === k).length; return k === "all" || n ? `<button class="fchip" data-actcat="${k}" aria-pressed="${actFilter === k}">${label} <span class="muted">${n}</span></button>` : ""; }).join("");
    return `<div class="act-section"><div class="act-bar"><h2>Eylemler <span class="hint">${ok.length} tanımlı</span></h2><div class="fchips2">${chips}</div></div>
      <div class="card"><div class="card-body tight" id="act-list">${ok.map(a => actionCard(a, p)).join("") || '<div class="act small muted">Eylem yok.</div>'}
        ${na.length ? `<details class="na"><summary>Bu sunucuya uygulanmaz (${na.length}): ${na.map(a => esc(a.id)).join(", ")}</summary>${na.map(a => actionCard(a, p)).join("")}</details>` : ""}</div></div></div>`;
  }

  function applyActFilter() {
    // seçili süzgeç yeniden çizimlerde (10 sn'lik tazeleme) korunur
    if (!$$("#srv-body [data-actcat]").some(b => b.dataset.actcat === actFilter)) actFilter = "all";
    $$("#act-list > .act[data-cat]").forEach(el => { el.hidden = actFilter !== "all" && el.dataset.cat !== actFilter; });
    $$("#srv-body [data-actcat]").forEach(b => b.setAttribute("aria-pressed", String(b.dataset.actcat === actFilter)));
  }

  /* ---------------- Yazılım kataloğu (paketin `items` bildirimi) ---------------- */
  // GENEL sayfa: component.json'da `items` (+ item_var) taşıyan HER paket için çizilir; sayfa belirli bir paket kimliğine bağlı değildir
  // (bkz. docs/package-standard.md). Kurulu/kurulmadı durumu son `durum` eyleminin `kurulu` ("kimlik[:mod]+kimlik…") ve `portlar`
  // ("kimlik:port+…") anahtarlarından gelir. Uyumluluk: `items` bildirmeyen kurulum 0.1.0 için LEGACY_ITEMS kullanılır.
  // Kur/Yükselt/Kaldır düğmeleri KAPALI: eylemler paket düzeyindedir ve önizleme önkoşulu öğeye özel tutulmaz;
  // öğeye özel çalıştırma (koşuya özel seçim + öğe başına kapı) gelene dek yalnız "Genel eylemler" çalışır.
  let storeFilter = "all", catVars = [], catSrv = "", catPkg = "";
  const LEGACY_ICONS = { docker: "🐳", nginx: "🌐", mariadb: "🗄️", redis: "⚡" };
  const LEGACY_ITEMS = [
    { id: "docker", label: "Docker Engine", description: "Konteyner çalışma ortamı ve Docker Compose eklentisi. Konteyner modundaki uygulamalar için gereklidir.", params: [], group: "Docker" },
    { id: "nginx", label: "Nginx", description: "HTTP ters proxy ve statik dosya sunucusu.", params: [], group: "nginx" },
    { id: "mariadb", label: "MariaDB", description: "MySQL uyumlu ilişkisel SQL veritabanı sunucusu.", params: [], group: "mariadb" },
    { id: "redis", label: "Redis", description: "Anahtar-değer veri önbelleği ve mesaj kuyruğu.", params: [], group: "redis" },
  ];
  const LEGACY_GEN = [["Kurulum modu", "kurulum_mod"], ["Ana klasör", "kurulum_ana_klasor"], ["Dışarıya açık", "kurulum_dis_erisim"], ["Eylem hedefi", "kurulum_yazilim"]];
  const splitPlus = v => (!v || v === "yok" ? [] : v.split("+").map(x => x.split(":")));
  const catVal = n => { const v = catVars.find(x => x.name === n); return v ? varCur(v) : ""; };

  // Katalog öğeleri: paketin `items` alanı; yoksa yalnız eski kurulum paketi için yerleşik liste; yoksa null (katalog sayfası yok).
  function catalogItems(p) {
    if (p.items && p.items.length) return p.items.map(it => ({ id: it.id, label: it.label || it.id, description: it.description || "", params: it.params || [], group: "" }));
    if (p.id === "kurulum") return LEGACY_ITEMS;
    return null;
  }

  async function loadCatVars(pkg) {
    catSrv = srvName; catPkg = pkg; // başarısız olsa da yeniden denemeyi bir sonraki tazelemeye bırak (döngü olmasın)
    try { catVars = (await json(`/api/vars?server=${encodeURIComponent(srvName)}&package=${encodeURIComponent(pkg)}`)).vars || []; } catch (e) { return; }
    if (view === "server" && srvTab === pkg && srv) renderServer();
  }

  function catalogStore(s, p, ok, na, items) {
    const legacy = !(p.items && p.items.length);
    const d = p.actions.find(a => a.id === "durum"), k = (d && d.keys) || {}, known = "kurulu" in k;
    const inst = Object.fromEntries(splitPlus(k.kurulu)), ports = Object.fromEntries(splitPlus(k.portlar));
    const dattr = d ? actBtnAttr(d, p) : "";
    const onN = items.filter(it => it.id in inst).length;
    const loaded = catSrv === srvName && catPkg === p.id;
    const varsOf = it => (it.params && it.params.length) ? catVars.filter(v => it.params.includes(v.name)) : catVars.filter(v => it.group && v.group === it.group);
    const short = (v, it) => { let n = v.name; if (n.startsWith(p.id + "_")) n = n.slice(p.id.length + 1); if (n.startsWith(it.id + "_")) n = n.slice(it.id.length + 1); return n; };
    const claimed = new Set();
    const cards = items.map(it => {
      const id = it.id, on = id in inst, mode = inst[id];
      const badge = !known ? '<span class="badge plain">durum okunmadı</span>' : on ? `<span class="badge ok">✔ Kurulu${mode ? " (" + esc(mode) + ")" : ""}</span>` : '<span class="badge plain">Kurulmadı</span>';
      const meta = [];
      if (on && ports[id]) meta.push(`Port: <strong>${esc(ports[id])}</strong>`);
      const vars = varsOf(it);
      vars.forEach(v => claimed.add(v.name));
      const groups = [...new Set(vars.map(v => v.group).filter(Boolean))];
      const rows = vars.map(v => `<div class="av-row"><span class="mono av-name" title="${esc(v.name)}">${esc(short(v, it))}</span><span class="chip">${esc(varCur(v)) || "(boş)"}</span>${v.set ? '<span class="badge ok">ayarlı</span>' : ""}</div>`).join("") || `<div class="small muted">${loaded ? "Değişken bulunamadı." : "Değişkenler yükleniyor…"}</div>`;
      const off = (label, tip) => `<button class="btn sm" disabled title="${tip}">${label}</button>`;
      const tip = "Öğeye özel çalıştırma yakında: önizleme önkoşulu her öğe için ayrı tutulacak. Şimdilik aşağıdaki Genel eylemler kullanılır.";
      const ico = esc(LEGACY_ICONS[id] || String(it.label).charAt(0).toUpperCase());
      return `<div class="card app-card" data-app="${esc(id)}" data-st="${!known ? "?" : on ? "on" : "off"}">
        <div class="app-top"><div class="app-ico" aria-hidden="true">${ico}</div><div class="app-titles"><div class="app-title"><span>${esc(it.label)}</span>${badge}</div><p class="app-desc">${esc(it.description)}</p></div></div>
        ${meta.length ? `<div class="app-meta">${meta.map(m => `<span class="meta-chip">${m}</span>`).join("")}</div>` : ""}
        <div class="app-vars"><div class="av-head"><span class="oz-title">Değişkenler</span><button class="link" data-kvars="${esc(groups.length === 1 ? groups[0] : "")}">Düzenle…</button></div>${rows}</div>
        <div class="app-foot"><span class="small muted">Eylemler yakında</span><div class="app-btns">${off("Önizle", tip)}${on ? off("Yükselt", tip) + off("Kaldır", tip) : off("Kur", tip)}</div></div></div>`;
    }).join("");
    const chip = (key, label, n) => `<button class="fchip" data-sf="${key}" aria-pressed="${storeFilter === key}">${label}${n === undefined ? "" : ` <span class="muted">${n}</span>`}</button>`;
    // genel ayarlar: eski kurulum için sabit dört değişken; diğerlerinde hiçbir öğeye ait olmayan değişkenler (en çok 8)
    const genPairs = legacy && p.id === "kurulum" ? LEGACY_GEN.map(([l, n]) => [l, n]) : catVars.filter(v => !claimed.has(v.name)).slice(0, 8).map(v => [v.name.startsWith(p.id + "_") ? v.name.slice(p.id.length + 1) : v.name, v.name]);
    const gen = genPairs.map(([l, n]) => `<div class="kv"><span class="muted">${esc(l)}</span><span class="chip" title="${esc(n)}">${esc(catVal(n)) || "—"}</span></div>`).join("") || '<div class="small muted">Ortak ayar yok.</div>';
    const itemVar = p.itemVar || (legacy && p.id === "kurulum" ? "kurulum_yazilim" : "");
    const sub = legacy ? "Kurulum motoru" : esc(p.name || p.id);
    return `<div class="store-head"><div><h2>Uygulama kataloğu</h2><div class="small muted">${sub} <span class="mono">v${esc(p.version)}</span> · bu USB'de.${legacy ? " Docker ya da yerel paket (native) olarak kurulur." : p.description ? " " + esc(p.description) : ""}</div></div>
        <div class="fchips2">${chip("all", "Tümü", items.length)}${known ? chip("on", "Kurulu", onN) + chip("off", "Kurulabilir", items.length - onN) : ""}</div></div>
      ${known ? "" : `<div class="notice" style="margin-bottom:14px"><span class="ico" aria-hidden="true">ℹ</span><div>Hangi uygulamaların kurulu olduğu henüz okunmadı. ${dattr ? `<button class="link" ${dattr}>Durumu oku</button> (salt okunur).` : ""}</div></div>`}
      <div class="store-grid" id="store-grid">${cards}</div>
      <div class="card gen-card"><div class="oz-top"><span class="oz-title">Genel ayarlar</span><span class="oz-tag">tüm uygulamalar</span></div>
        <div class="gen-grid">${gen}</div>
        <div class="oz-foot oz-foot-row"><span>${known && dattr ? `<button class="link" ${dattr}>Durumu yenile</button> · ` : ""}Son durum: ${d && d.lastTime ? esc(ago(d.lastTime)) + " önce" : "okunmadı"}</span><button class="link" data-kvars-all>Tüm değişkenler…</button></div></div>
      <details class="card store-actions"><summary><b>Genel eylemler</b> <span class="small muted">durum, preview, kur, yukselt, kaldir${itemVar ? ` · <span class="mono">${esc(itemVar)}</span> seçimine göre çalışır` : ""}</span></summary>${actionsSection(ok, na, p)}</details>`;
  }

  function applyStoreFilter() {
    $$("#store-grid .app-card").forEach(el => { el.hidden = storeFilter !== "all" && el.dataset.st !== storeFilter; });
    $$("#srv-body [data-sf]").forEach(b => b.setAttribute("aria-pressed", String(b.dataset.sf === storeFilter)));
  }

  function bindCatalog(pkg) {
    $$("#srv-body [data-sf]").forEach(b => b.addEventListener("click", () => { storeFilter = b.dataset.sf; applyStoreFilter(); }));
    $$("#srv-body [data-kvars]").forEach(b => b.addEventListener("click", () => openVars(srvName, pkg, b.dataset.kvars || undefined)));
    $("#srv-body [data-kvars-all]")?.addEventListener("click", () => openVars(srvName, pkg));
    $$("#srv-body [data-actcat]").forEach(b => b.addEventListener("click", () => { actFilter = b.dataset.actcat; applyActFilter(); }));
    if (!$$("#srv-body [data-sf]").some(b => b.dataset.sf === storeFilter)) storeFilter = "all";
    applyStoreFilter(); applyActFilter();
    if (catSrv !== srvName || catPkg !== pkg) loadCatVars(pkg);
  }

  function renderServer() {
    if (!srv) return;
    const s = srv.server;
    const stCls = !s.valid ? "warn" : !s.checked ? "plain" : s.up ? "ok" : s.knock || s.conn === "refused" ? "warn" : "danger";
    const addr = s.valid ? `${s.user}@${s.host}:${s.port}` : "";
    $("#srv-head").innerHTML = `<nav class="sv-crumb" aria-label="Konum"><a href="#" id="srv-back">Sunucular</a><span aria-hidden="true">/</span><span>${esc(s.name)}</span></nav>
      <div class="sv-bar">
        <div class="sv-id">${osIcon(s.os, 34)}<h1>${esc(s.name)}</h1>
          ${s.platform ? `<span class="tag">${esc(s.platform)}</span>` : ""}
          <span class="sv-state ${stCls}"><span class="sv-dot" aria-hidden="true"></span>${esc(stateText(s))}</span>
          ${s.valid ? `<button class="sv-chip" id="srv-addr" type="button" title="Adresi kopyala"><span class="mono">${esc(addr)}</span>${COPY_ICON}</button>` : '<span class="small muted">server.env geçersiz</span>'}
          <button class="sv-edit" id="srv-edit" type="button" title="IP, kullanıcı ve portu düzenle" aria-label="Bağlantı bilgilerini düzenle"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z"/></svg></button></div>
        <div class="sv-actions">
          <button class="btn primary" id="srv-term" ${s.valid ? "" : "disabled"}>Terminal'de bağlan</button>
          <button class="btn" id="srv-iterm" ${s.valid ? "" : "disabled"} title="Tarayıcıda gömülü terminal (ssh -tt)">Gömülü terminal</button>
          <button class="btn" id="srv-copy" ${s.valid ? "" : "disabled"} title="knock etkinse önce knock gönderir">SSH komutu</button>
          <button class="btn icon-only" id="srv-more" type="button" aria-haspopup="menu" aria-label="Daha fazla işlem" title="Yeniden adlandır, sil, günlükler…"><svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><circle cx="5" cy="12" r="1.8"/><circle cx="12" cy="12" r="1.8"/><circle cx="19" cy="12" r="1.8"/></svg></button>
        </div>
      </div>`;
    $("#srv-back").addEventListener("click", e => { e.preventDefault(); showView("servers"); });
    $("#srv-term").addEventListener("click", () => act(s.name, "terminal", "Terminal'de açıldı"));
    $("#srv-iterm").addEventListener("click", () => window.KeyLabTerminal.open(s.name));
    $("#srv-copy").addEventListener("click", async () => { const r = await json("/api/action", { name: s.name, action: "cmd" }); try { await navigator.clipboard.writeText(r.cmd); toast("Komut kopyalandı"); } catch { prompt("Kopyalayın:", r.cmd); } });
    $("#srv-addr")?.addEventListener("click", async () => { try { await navigator.clipboard.writeText(addr); toast("Adres kopyalandı"); } catch { prompt("Kopyalayın:", addr); } });
    $("#srv-edit").addEventListener("click", () => openEditAddr(s.name));
    $("#srv-more").addEventListener("click", e => { e.stopPropagation(); const r = e.currentTarget.getBoundingClientRect(); openCtx(r.right - 230, r.bottom + 6, s.name); });

    const ti = id => { const i = TAB_ORDER.indexOf(id); return i < 0 ? 99 : i; };
    const pk = [...srv.packages].sort((a, b) => ti(a.id) - ti(b.id));
    const tabs = [{ id: "ozet" }, ...pk];
    if (!tabs.some(p => p.id === srvTab)) srvTab = "ozet";
    $("#srv-tabs").innerHTML = tabs.map(p => `<button role="tab" id="tab-${esc(p.id)}" aria-selected="${p.id === srvTab}" data-p="${esc(p.id)}">${esc(p.id === "ozet" ? "Özet" : TAB_LABEL[p.id] || p.name || p.id)}</button>`).join("");
    $$("#srv-tabs button").forEach(b => b.addEventListener("click", () => { srvTab = b.dataset.p; history.replaceState(null, "", `#s=${encodeURIComponent(srvName)}&p=${encodeURIComponent(srvTab)}`); renderServer(); }));

    const p = pk.find(x => x.id === srvTab);
    if (srvTab === "ozet") { renderOzet(s, pk); return; }
    setTimeout(() => $$("#srv-body [data-vars]").forEach(b => b.addEventListener("click", () => openVars(srvName, b.dataset.vars))), 0);
    setTimeout(() => $$("#srv-body [data-write]").forEach(b => b.addEventListener("click", () => { const [pid, aid] = b.dataset.write.split("|"); openWrite(srvName, pid, aid); })), 0);
    setTimeout(() => $$("#srv-body [data-term]").forEach(b => b.addEventListener("click", () => { const [pid, aid] = b.dataset.term.split("|"); openTerm(srvName, pid, aid); })), 0);
    setTimeout(() => $$("#srv-body [data-run]").forEach(b => b.addEventListener("click", () => { const [pid, aid] = b.dataset.run.split("|"); openRun(srvName, pid, aid); })), 0);
    if (!p) { $("#srv-body").innerHTML = '<div class="card"><div class="card-body small muted">Bu sunucu için paket bulunamadı.</div></div>'; return; }
    const ok = p.actions.filter(a => a.applicable), na = p.actions.filter(a => !a.applicable);
    const cat = catalogItems(p);
    if (cat) { $("#srv-body").innerHTML = catalogStore(s, p, ok, na, cat); bindCatalog(p.id); return; }
    $("#srv-body").innerHTML = `${p.id === "linux-hardening" ? lynisHero(s, p) : ""}<div class="pk-top">${pkgInfoCard(p, ok)}${varsCard(p)}</div>${statusCard(p)}${actionsSection(ok, na, p)}`;
    $$("#srv-body [data-actcat]").forEach(b => b.addEventListener("click", () => { actFilter = b.dataset.actcat; applyActFilter(); }));
    $("#srv-body [data-go-lynis-p]")?.addEventListener("click", () => goLynis(srvName, s.lynis && s.lynis.warnings > 0 ? "uyari" : "all"));
    applyActFilter();
  }

  async function act(name, action, msg) {
    try { const r = await json("/api/action", { name, action }); toast(r && r.knock ? "Terminal açıldı · 🚪 knock Terminal'de gönderiliyor" : msg); }
    catch (e) { toast("Hata: " + e.message); }
  }

  async function openServer(name, tab) {
    srvName = name; srvTab = tab || "ozet";
    srv = null;
    showView("server"); renderSrvTree();
    history.replaceState(null, "", `#s=${encodeURIComponent(name)}&p=${encodeURIComponent(srvTab || "")}`);
    $("#srv-head").innerHTML = `<button class="back" id="srv-back">← Sunucular</button><div class="srv-title"><h1>${esc(name)}</h1></div>`;
    $("#srv-back").addEventListener("click", () => showView("servers"));
    $("#srv-tabs").innerHTML = ""; $("#srv-body").innerHTML = '<div class="small muted">yükleniyor…</div>';
    // Yanıt gelene kadar kullanıcı başka sunucuya/sayfaya geçmiş olabilir: eski yanıt yenisini ezmesin.
    let data;
    try { data = await json("/api/server?name=" + encodeURIComponent(name)); }
    catch (e) { if (srvName === name && view === "server") $("#srv-body").innerHTML = `<div class="notice danger"><span class="ico" aria-hidden="true">✕</span><div>Sunucu açılamadı: ${esc(e.message)}</div></div>`; return; }
    if (srvName !== name || view !== "server") return;
    srv = data;
    renderServer();
  }

  $$(".mode-toggle button").forEach(b => b.addEventListener("click", () => {
    mode = b.dataset.mode;
    try { localStorage.setItem("sshpocket.mode", mode); } catch (e) { /* önemli değil */ }
    render();
  }));

  /* ---------------- Arama ---------------- */
  const search = $("#search-input");
  search.addEventListener("input", () => { query = search.value.trim().toLowerCase(); render(); });
  document.addEventListener("keydown", e => {
    if (e.key === "/" && document.activeElement !== search && !/input|textarea/i.test(document.activeElement.tagName)) { e.preventDefault(); search.focus(); }
  });

  /* ---------------- Bağlam menüsü ---------------- */
  const ctx = $("#ctx"); let ctxName = "";
  function openCtx(x, y, name) {
    ctxName = name; $("#ctx-head").textContent = name;
    ctx.classList.add("open");
    const r = ctx.getBoundingClientRect();
    ctx.style.left = Math.max(8, Math.min(x, innerWidth - r.width - 8)) + "px";
    ctx.style.top = Math.max(8, Math.min(y, innerHeight - r.height - 8)) + "px";
    const f = ctx.querySelector("button"); if (f) f.focus();
  }
  const closeCtx = () => ctx.classList.remove("open");
  function bindServers() {
    $$(".stile[data-add]").forEach(el => {
      el.addEventListener("click", () => showView("add"));
      el.addEventListener("keydown", e => { if (e.key === "Enter") showView("add"); });
    });
    $$(".server[data-name], .stile[data-name]").forEach(el => {
      el.addEventListener("click", () => openServer(el.dataset.name));
      el.addEventListener("keydown", e => { if (e.key === "Enter") openServer(el.dataset.name); });
      el.addEventListener("contextmenu", e => { e.preventDefault(); openCtx(e.clientX, e.clientY, el.dataset.name); });
      el.addEventListener("keydown", e => {
        if (e.key === "F10" || e.key === "ContextMenu") { e.preventDefault(); const r = el.getBoundingClientRect(); openCtx(r.left + 40, r.bottom + 4, el.dataset.name); }
      });
      const m = el.querySelector('[data-act="menu"]');
      if (m) m.addEventListener("click", e => { e.stopPropagation(); const r = m.getBoundingClientRect(); openCtx(r.right - 240, r.bottom + 4, el.dataset.name); });
    });
  }
  document.addEventListener("click", e => { if (!ctx.contains(e.target)) closeCtx(); });
  document.addEventListener("keydown", e => { if (e.key === "Escape") { closeCtx(); closeWizard(); closeRun(); closeVars(); } });
  ctx.addEventListener("keydown", e => {
    const items = $$("button", ctx), i = items.indexOf(document.activeElement);
    if (e.key === "ArrowDown") { e.preventDefault(); items[(i + 1) % items.length].focus(); }
    if (e.key === "ArrowUp") { e.preventDefault(); items[(i - 1 + items.length) % items.length].focus(); }
  });
  $$("#ctx button").forEach(b => b.addEventListener("click", async () => {
    const a = b.dataset.ctx, name = ctxName; closeCtx();
    try {
      if (a === "copy") { const r = await json("/api/action", { name, action: "cmd" }); try { await navigator.clipboard.writeText(r.cmd); toast("Komut kopyalandı"); } catch { prompt("Kopyalayın:", r.cmd); } }
      else if (a === "term") await act(name, "terminal", "Terminal'de açıldı");
      else if (a === "refresh") { await load(); toast("Yenilendi"); }
      else if (a === "logs") goLogs(name);
      else if (a === "lynis") goLynis(name, "");
      else if (a === "draft") openWizard(name);
      else if (a === "edit") openEditAddr(name);
      else if (a === "rename") openRename(name);
      else if (a === "delete") openDelete(name);
    } catch (err) { toast("Hata: " + err.message); }
  }));
  $("#refresh-all").addEventListener("click", async () => { const b = $("#refresh-all"); b.disabled = true; await load(); b.disabled = false; toast("Yenilendi"); });

  /* ---------------- Sihirbaz (TASLAK: komut çalıştırmaz) ---------------- */
  const wiz = $("#wizard"), confirmInput = $("#confirm-input"), nextBtn = $("#next-btn"), applyBtn = $("#apply-btn"), finishBtn = $("#finish-btn");
  let wname = "";
  function setStep(n) {
    [1, 2, 3].forEach(i => ($("#step-" + i).hidden = i !== n));
    $$("#wiz-steps .step").forEach((t, i) => { t.classList.toggle("active", i + 1 === n); t.classList.toggle("done", i + 1 < n); });
    nextBtn.hidden = n !== 1; applyBtn.hidden = n !== 2; finishBtn.hidden = n !== 3;
    if (n === 2) { confirmInput.value = ""; applyBtn.disabled = true; setTimeout(() => confirmInput.focus(), 40); }
  }
  function openWizard(name) {
    wname = name;
    const s = home.servers.find(x => x.name === name), pkg = (s && s.last && s.last.package) || (home.packages[0] && home.packages[0].id) || "?";
    const ver = (home.packages.find(p => p.id === pkg) || {}).version || "?";
    $("#wiz-desc").textContent = `Paket: ${pkg} · Sunucu: ${name}`;
    $("#w-pkg").textContent = `ad:     ${pkg}\nsürüm:  ${ver}\nkaynak: USB`;
    $("#w-srv").textContent = `ad:     ${name}\ndurum:  ${s && s.up ? "çevrimiçi" : "bağlantı yok"}`;
    $("#wiz-confirm-name").textContent = name; confirmInput.placeholder = name;
    setStep(1); wiz.classList.add("open"); setTimeout(() => nextBtn.focus(), 40);
  }
  function closeWizard() { wiz.classList.remove("open"); }
  $$("[data-close]").forEach(b => b.addEventListener("click", closeWizard));
  nextBtn.addEventListener("click", () => setStep(2));
  confirmInput.addEventListener("input", () => { applyBtn.disabled = confirmInput.value.trim() !== wname; });
  applyBtn.addEventListener("click", () => { if (confirmInput.value.trim() === wname) setStep(3); });

  let rz = 0;
  window.addEventListener("resize", () => {
    clearTimeout(rz);
    rz = setTimeout(() => { if (view === "lynis" && lynData) paintLynis(); else if (view === "server" && srv) renderServer(); }, 250);
  });

  /* ---------------- Başlangıç ---------------- */
  json("/api/about").then(a => { askAvail = !!a.askpass; paintAsk(); }).catch(() => {});
  const startHash = new URLSearchParams(location.hash.slice(1)); // showView adres çubuğundaki #s=… kısmını siler: ÖNCE oku
  showView("dash");
  load().then(() => {
    if (startHash.get("s")) openServer(startHash.get("s"), startHash.get("p") || "");
  });
  setInterval(() => {
    if (document.hidden || wiz.classList.contains("open")) return;
    load();
    if (view === "server" && srvName) json("/api/server?name=" + encodeURIComponent(srvName)).then(r => { srv = r; renderServer(); }).catch(() => {});
  }, 10000);
})();
