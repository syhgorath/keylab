import { $, $$, t, esc, api, json, toast, ago, localTime, isOk, resHtml } from "./js/lib/util.js";
import { initShortcuts } from "./js/ui/shortcuts.js";
import { initContextMenu } from "./js/ui/context-menu.js";
import { initSshImport } from "./js/ui/ssh-import.js";
import { initWizard } from "./js/ui/wizard.js";
import { initKnock } from "./js/ui/knock.js";
import { initJobs } from "./js/ui/jobs.js";
import { initLynis, chart } from "./js/ui/lynis.js";
import { initLogs } from "./js/ui/logs.js";
import { initPackages } from "./js/ui/packages.js";
import { OS_LABEL, osIcon } from "./js/lib/icons.js";
import { TAB_LABEL } from "./js/lib/labels.js";
import { appState } from "./js/state.js";

(function () {
  "use strict";


  const matches = (...parts) => !appState.query || parts.join(" ").toLowerCase().includes(appState.query);

  /* ---------------- Gezinme ---------------- */
  // Adres çubuğu = konum: "" (pano) · #v=<sayfa> · #s=<sunucu>&p=<sekme>. Yeni sayfa/sunucu geçmişe EKLENİR (geri/ileri çalışır);
  // aynı sunucudaki sekme değişimi ve geçmişten uygulanan gezinme geçmişi ÇOĞALTMAZ (replace).
  const VIEWS = ["dash", "servers", "packages", "compare", "reports", "lynis", "logs", "settings", "add"];
  function setHash(h, replace) {
    if (location.hash === h || (h === "" && location.hash === "")) return;
    try { (replace || appState.routing ? history.replaceState : history.pushState).call(history, null, "", h || location.pathname); } catch (e) { /* adres çubuğu güncellenemezse sayfa yine çalışır */ }
  }
  function applyHash() {
    const q = new URLSearchParams(location.hash.slice(1));
    appState.routing = true;
    try {
      if (q.get("s")) { if (!(appState.view === "server" && appState.srvName === q.get("s"))) openServer(q.get("s"), q.get("p") || ""); else if (q.get("p") && q.get("p") !== appState.srvTab) { appState.srvTab = q.get("p"); renderServer(); } }
      else showView(VIEWS.includes(q.get("v")) ? q.get("v") : "dash");
    } finally { appState.routing = false; }
  }
  window.addEventListener("popstate", applyHash); // geri/ileri ve elle yazılan #… (hashchange ile aynı olay zinciri)

  function showView(name) {
    appState.view = name;
    $$(".view").forEach(v => (v.hidden = v.id !== "view-" + name));
    const navName = name === "server" || name === "add" ? "servers" : name;
    $$(".nav-item[data-nav]").forEach(b => b.setAttribute("aria-current", b.dataset.nav === navName ? "page" : "false"));
    if (name !== "server") setHash(name === "dash" ? "" : "#v=" + name);
    $(".aside").style.display = name === "dash" ? "" : "none";
    const app = $(".app");
    app.classList.toggle("v-wide", name !== "dash");
    app.classList.toggle("v-server", name === "server");
    $("#srvtree").hidden = name !== "server";
    if (name === "logs") logs.load();
    if (name === "lynis") lynis.load();
    if (name === "packages") packages.load();
    if (name === "add") initAdd();
    if (name === "settings") loadSettings();
    if (name === "compare") window.KeyLabCompare.load(false);
    if (name === "reports") window.KeyLabReports.load();
  }
  $$(".nav-item[data-nav]").forEach(b => b.addEventListener("click", () => showView(b.dataset.nav)));
  $$("[data-nav-target]").forEach(b => b.addEventListener("click", () => showView(b.dataset.navTarget)));

  /* ---------------- Çizimler ---------------- */
  function serverRow(s) {
    const tag = s.platform ? `<span class="tag">${esc(s.platform)}</span>` : "";
    const last = s.last
      ? `${resHtml(s.last)} <span class="muted mono" title="${esc(localTime(s.last.time))}">${esc(ago(s.last.time))}</span>`
      : '<span class="res none">— kayıt yok</span>';
    const sub = s.last ? `<div class="srv-addr sj-7a21c6">${esc(s.last.package)} ${esc(s.last.version)} · ${esc(s.last.action)}</div>` : "";
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
  try { appState.mode = localStorage.getItem("sshpocket.mode") === "list" ? "list" : "tiles"; } catch (e) { /* tarayıcı depolaması kapalı olabilir */ }

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
  const emptyServers = '<li class="server sj-2a9295"><div class="srv-ident"><div class="srv-name">Sunucu bulunamadı</div><div class="srv-addr">USB\'de servers/&lt;ad&gt;/server.env yok ya da arama eşleşmedi</div></div></li>';

  function pkgRow(p) {
    return `<div class="pkg-row"><div><div class="pkg-name">${esc(p.id)}</div>
      <div class="pkg-meta"><span class="ver">${esc(p.version)}</span><span class="muted">· USB'de kurulu sürüm</span></div></div>
      <div class="pkg-actions"><button class="btn sm ghost" disabled title="Güncelleme denetimi bu taslakta yok">güncelleme denetimi yok</button></div></div>`;
  }

  function kpis() {
    const sv = appState.home.servers, up = sv.filter(s => s.checked && s.up).length, chk = sv.filter(s => s.checked).length, keys = sv.filter(s => s.hasKey).length;
    const day = Date.now() - 864e5;
    const fails = sv.flatMap(s => (s.recent || []).filter(a => !isOk(a.result) && Date.parse(a.time) > day).map(a => ({ ...a, server: s.name })));
    const ly = sv.filter(s => s.lynis);
    const avg = ly.length ? Math.round(ly.reduce((t, s) => t + s.lynis.score, 0) / ly.length) : null;
    const gain = ly.reduce((t, s) => t + (s.lynis.score - s.lynis.history[0]), 0);
    // renk tonu (sol şerit + değer rengi): durumun yalnız rengi taşımaması için nokta ve alt metin de kalır
    const tOn = !sv.length || !chk ? "" : up === sv.length ? "ok" : up === 0 ? "danger" : "warn";
    const tFail = sv.length ? (fails.length ? "danger" : "ok") : "";
    const tLy = avg === null ? "" : avg >= 80 ? "ok" : avg >= 60 ? "warn" : "danger";
    const tn = t => (t ? ` data-tone="${t}"` : "");
    return `
      <div class="kpi"${tn(tOn)}><div class="kpi-label"><span class="dot ${chk && up === sv.length ? "ok" : "warn"}" aria-hidden="true"></span>Çevrimiçi</div>
        <div class="kpi-value">${sv.some(s => s.valid) && !chk ? "…" : up}<small>/${sv.length}</small></div>
        <div class="kpi-sub"><span class="ok">✔</span> anahtar var: ${keys} · <span class="muted">yok: ${sv.length - keys}</span></div></div>
      <div class="kpi"><div class="kpi-label"><span class="dot ok" aria-hidden="true"></span>Kurulu paket</div>
        <div class="kpi-value">${appState.home.packages.length}</div>
        <div class="kpi-sub">${esc(appState.home.packages.map(p => p.id).join(" · ")) || "—"}</div></div>
      <div class="kpi"${tn(tFail)}><div class="kpi-label"><span class="dot ${fails.length ? "danger" : "ok"}" aria-hidden="true"></span>Son 24 sa başarısız</div>
        <div class="kpi-value">${fails.length}</div>
        <div class="kpi-sub">${fails.length ? `<span class="danger">${esc(fails[0].server)}</span> · ${esc(fails[0].package)} ${esc(fails[0].action)}` : "başarısız eylem yok"}</div></div>
      <div class="kpi"${tn(tLy)}><div class="kpi-label"><span class="dot ${avg === null ? "warn" : "ok"}" aria-hidden="true"></span>Ortalama Lynis</div>
        <div class="kpi-value">${avg === null ? "—" : avg}<small>${avg === null ? "" : "/100"}</small></div>
        <div class="kpi-sub">${avg === null ? "ölçüm yok" : `${ly.length} sunucu · ${gain >= 0 ? "+" : ""}${gain} toplam değişim`}</div></div>`;
  }

  function pills() {
    const sv = appState.home.servers, up = sv.filter(s => s.checked && s.up).length, off = sv.filter(s => s.checked && !s.up).length;
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
      <div class="lynis-head"><div class="lynis-score">${l.score}</div><div class="small muted sj-9da19d">/100</div>
        ${l.history.length > 1 ? `<div class="lynis-delta">${d >= 0 ? "▲ +" : "▼ "}${d}</div>` : ""}</div>
      <div class="lynis-meta"><span class="badge warn">uyarı ${l.warnings}</span><span class="badge info">öneri ${l.suggestions}</span>
        ${l.history.length > 1 ? `<span class="badge plain">${l.history[0]} → ${l.score}</span>` : ""}</div>
      ${big ? `<div class="chartbox">${chart(l)}</div>` : spark(l.history)}
      <div class="small muted sj-0f5c58"><span>${l.history.length} ölçüm</span><span>${esc((l.date || "").slice(0, 10))}</span></div>`;
  }

  function timeline() {
    const all = appState.home.servers.flatMap(s => (s.recent || []).map(a => ({ ...a, server: s.name })))
      .sort((a, b) => Date.parse(b.time) - Date.parse(a.time)).slice(0, 7);
    if (!all.length) return '<li class="small muted">Henüz kayıt yok.</li>';
    return all.map(a => `<li class="${isOk(a.result) ? "ok" : "fail"}"><div class="tl-time" title="${esc(localTime(a.time))}">${esc(ago(a.time))} önce</div>
      <div class="tl-text"><b>${esc(a.server)}</b> · ${esc(a.package)} ${esc(a.version)} · ${esc(a.action)} · <span class="${isOk(a.result) ? "c-ok" : "c-danger"}">${isOk(a.result) ? "başarılı" : "başarısız"}</span></div></li>`).join("");
  }

  /* Sunucu sayfasının ortadaki sunucu listesi (Proxmox tarzı kaynak ağacı) */
  let treeQ = "";
  function renderSrvTree() {
    const el = $("#srvtree-list"); if (!el) return;
    const sv = appState.home.servers.filter(s => !treeQ || `${s.name} ${s.host || ""}`.toLowerCase().includes(treeQ));
    $("#srvtree-title").textContent = `${t("tree_title", "Sunucular")} (${appState.home.servers.length})`;
    el.innerHTML = sv.map(s => `<li><button class="tree-item${s.name === appState.srvName ? " on" : ""}" data-tsrv="${esc(s.name)}" aria-current="${s.name === appState.srvName ? "page" : "false"}"><span class="status-dot ${dotState(s)}" aria-label="durum: ${stateText(s)}"></span><span class="tree-txt"><span class="tree-name">${esc(s.name)}</span><span class="tree-addr mono">${esc(s.host || "")}</span></span></button></li>`).join("") || '<li class="small muted sj-cfa1ec">${t("tree_none", "Eşleşen sunucu yok.")}</li>';
  }
  $("#srvtree-list").addEventListener("click", e => { const b = e.target.closest("[data-tsrv]"); if (b && b.dataset.tsrv !== appState.srvName) openServer(b.dataset.tsrv, appState.srvTab); });
  $("#srvtree-q").addEventListener("input", e => { treeQ = e.target.value.trim().toLowerCase(); renderSrvTree(); });
  $("#srvtree-add").addEventListener("click", () => showView("add"));
  /* Dil: düğme TR/EN arasında geçer; dinamik çizimler yeniden yapılır */
  $("#lang-btn").addEventListener("click", () => { const L = window.KeyLabI18n; L.setLang(L.getLang() === "tr" ? "en" : "tr"); });
  window.addEventListener("keylab:langchange", () => { render(); renderSrvTree(); if (appState.view === "server" && appState.srv) renderServer(); });

  /* Sunucu süzgeci (Tümü / Çevrimiçi / Kapalı / Uyarılı / Anahtarsız / Knock): home.servers değişmez, yalnız görünüm süzülür */
  let fleetFilter = "all";
  let visibleServers = []; // süzgeç + arama sonrası listede görünen geçerli sunucular (toplu işlem hedefi)
  const hasWarn = s => !!((s.lynis && s.lynis.warnings > 0) || (s.recent || []).some(a => !isOk(a.result)));
  const FLEET_TESTS = {
    all: () => true,
    up: s => s.checked && s.up,
    down: s => s.checked && !s.up,
    warn: hasWarn,
    nokey: s => s.valid && !s.hasKey,
    knock: s => !!s.knock,
  };
  function renderFleetFilter() {
    const defs = [["all", t("ff_all", "Tümü"), true], ["up", t("ff_up", "Çevrimiçi"), true], ["down", t("ff_down", "Kapalı"), true], ["warn", "⚠ " + t("ff_warn", "Uyarılı"), true], ["nokey", t("ff_nokey", "Anahtarsız"), false], ["knock", "🔒 knock", false]];
    const html = defs.map(([k, label, always]) => {
      const n = appState.home.servers.filter(FLEET_TESTS[k]).length;
      return always || n || fleetFilter === k ? `<button class="fchip" data-ff="${k}" aria-pressed="${fleetFilter === k}">${label} <span class="muted">${n}</span></button>` : "";
    }).join("");
    $$(".fleet-filter").forEach(el => { el.innerHTML = appState.home.servers.length ? html : ""; });
  }
  document.addEventListener("click", e => {
    const b = e.target.closest(".fleet-filter [data-ff]"); if (!b) return;
    fleetFilter = b.dataset.ff; render();
  });
  function render() {
    renderSrvTree();
    const sv = appState.home.servers.filter(s => matches(s.name, s.platform, s.last && s.last.package)).filter(FLEET_TESTS[fleetFilter] || FLEET_TESTS.all);
    visibleServers = sv.filter(s => s.valid).map(s => s.name);
    renderFleetFilter();
    const pk = appState.home.packages.filter(p => matches(p.id));
    const addTile = '<li class="stile add" tabindex="0" data-add="1" aria-label="Sunucu ekle"><div class="plus" aria-hidden="true">+</div><div>Sunucu ekle</div></li>';
    const rows = (sv.map(appState.mode === "tiles" ? tile : serverRow).join("") || emptyServers) + (appState.mode === "tiles" && !appState.query && fleetFilter === "all" ? addTile : "");
    for (const id of ["#servers", "#servers-all"]) { $(id).innerHTML = rows; $(id).classList.toggle("tiles", appState.mode === "tiles"); }
    $$(".mode-toggle button").forEach(b => b.setAttribute("aria-pressed", String(b.dataset.mode === appState.mode)));
    $("#servers-count").textContent = appState.home.servers.length + " kayıt";
    $("#pkgs-dash").innerHTML = pk.map(pkgRow).join("") || '<div class="small muted sj-cc781b">Paket bulunamadı.</div>';
    $("#kpis").innerHTML = kpis();
    $("#pills").innerHTML = pills();
    $("#n-servers").textContent = appState.home.servers.length;
    $("#n-packages").textContent = appState.home.packages.length;
    if (window.KeyLabDock) window.KeyLabDock.paint();
    if (window.KeyLabOnboarding) window.KeyLabOnboarding.render(appState.home.servers.length === 0);
    $("#idx-serial").textContent = "seri: " + (appState.home.indexSerial || "—");
    if (!appState.selected || !appState.home.servers.some(s => s.name === appState.selected)) {
      const measured = appState.home.servers.filter(s => s.lynis).sort((a, b) => (b.lynis.date || "").localeCompare(a.lynis.date || ""));
      const first = measured[0] || appState.home.servers[0];
      appState.selected = first ? first.name : "";
    }
    $("#a-lynis").innerHTML = lynisCard(appState.home.servers.find(s => s.name === appState.selected));
    $("#a-tl").innerHTML = timeline();
    bindServers();
    $("#brand-sub").textContent = "yerel · taslak";
    if (appState.view === "compare") window.KeyLabCompare.load(false); // durum noktaları ve yeni sunucular
  }

  async function load() {
    try { appState.home = await json("/api/home"); appState.home.servers = appState.home.servers || []; appState.home.packages = appState.home.packages || []; render(); }
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
      appState.home.servers.forEach(s => { if (s.name in st) { s.up = st[s.name].up; s.conn = st[s.name].conn; s.checked = true; } });
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
      <div><div class="vname">${esc(v.name)} ${v.set ? '<span class="badge ok">ayarlı</span>' : v.required && !v.default ? '<span class="badge danger">gerekli</span>' : '<span class="badge plain">varsayılan</span>'} <span class="chip sj-11a508">${esc(v.type)}</span></div>
        <div class="vdesc small muted" title="Tamamını görmek için tıklayın">${esc(v.description)}</div>
        ${v.warnIf ? `<div class="small sj-0e2fcf">⚠ uyarı eşiği: ${esc(v.warnIf)}</div>` : ""}</div>
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
      window.KeyLabCompare.invalidate(); if (appState.view === "compare") window.KeyLabCompare.load(false);
      try { appState.srv = await json("/api/server?name=" + encodeURIComponent(vSrv)); if (appState.view === "server") renderServer(); } catch (e) { /* bir sonraki turda */ }
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

  /* ---------------- İş izleyici + çalıştırma penceresi (js/ui/jobs.js) ---------------- */
  const jobs = initJobs({
    load: () => load(), renderServer: () => renderServer(), showView: n => showView(n), loadPackages: () => packages.load(), checkUpdatesUI: () => packages.checkUpdates(),
    renderAddSteps: () => renderAddSteps(), markTaskDone: k => addDone.add(k), getRemoval: () => ({ name: delName, purge: delPurge }), openLastLog: (p, sv) => logs.openLast(p, sv),
  });
  const { openRun, openWrite, openTerm, jobBusy, watchJob, closeRun, openJob, adoptJob } = jobs;

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
  $$("#set-mode button").forEach(b => b.addEventListener("click", () => { appState.mode = b.dataset.v; try { localStorage.setItem("sshpocket.mode", appState.mode); } catch (e) { /* önemli değil */ } render(); paintMode(); }));
  const paintMode = () => $$("#set-mode button").forEach(x => x.setAttribute("aria-pressed", String(x.dataset.v === appState.mode)));
  applyTheme();
  const paintAsk = () => $$("#set-ask button").forEach(x => { x.setAttribute("aria-pressed", String(x.dataset.v === appState.askMode)); if (x.dataset.v === "system") { x.disabled = !appState.askAvail; x.title = appState.askAvail ? "" : "Bu platformda sistem penceresi yok"; } });
  /* Eylemler > aynı anda en çok (sunucu tarafı ayar: /api/ui-settings) */
  const maxEl = $("#set-maxjobs"), maxSt = $("#set-maxjobs-st");
  let maxShown = 4;
  async function loadMaxJobs() { try { const r = await json("/api/ui-settings"); maxShown = r.maxJobs; maxEl.value = r.maxJobs; maxEl.min = r.min; maxEl.max = r.max; } catch (e) { maxSt.textContent = "okunamadı: " + e.message; } }
  maxEl.addEventListener("change", async () => {
    const n = parseInt(maxEl.value, 10), lo = parseInt(maxEl.min, 10), hi = parseInt(maxEl.max, 10);
    if (!(n >= lo && n <= hi)) { maxEl.value = maxShown; maxSt.textContent = `${lo} ile ${hi} arasında olmalı`; return; }
    try { const r = await json("/api/ui-settings", { maxJobs: n }); maxShown = r.maxJobs; maxEl.value = r.maxJobs; maxSt.textContent = "kaydedildi"; }
    catch (e) { maxEl.value = maxShown; maxSt.textContent = "kaydedilemedi: " + e.message; }
  });
  loadMaxJobs();
  $$("#set-ask button").forEach(b => b.addEventListener("click", () => { appState.askMode = b.dataset.v; try { localStorage.setItem("sshpocket.askpass", appState.askMode); } catch (e) { /* önemli değil */ } paintAsk(); }));

  async function loadSettings() {
    paintMode(); applyTheme(); paintAsk();
    let a;
    try { a = await json("/api/about"); appState.askAvail = !!a.askpass; paintAsk(); } catch (e) { $("#set-paths").innerHTML = `<div class="notice danger"><span class="ico" aria-hidden="true">✕</span><div>Yüklenemedi: ${esc(e.message)}</div></div>`; return; }
    const row = (k, v) => `<div class="urow sj-8657c7"><div class="muted">${esc(k)}</div><div class="mono small sj-752397">${esc(v) || "—"}</div></div>`;
    $("#set-paths").innerHTML = [["KeyLab sürümü", a.version], ["Kök klasör (USB)", a.root], ["Sunucular", a.serversDir], ["Paketler", a.componentsDir], ["Durum ve günlükler", a.stateDir], ["Platform", a.platform], ["Go", a.go]].map(([k, v]) => row(k, v)).join("");
    const conf = Object.entries(a.conf || {});
    $("#set-conf").innerHTML = conf.length ? conf.map(([k, v]) => row(k, v)).join("") : '<div class="small muted sj-fdf20b">sshpocket.conf bulunamadı ya da boş.</div>';
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
        <ul class="small muted sj-dc5781"><li>Klasör ve anahtar dosyaları (<span class="mono">id_ed25519_${esc(name)}</span>) yeni ada taşınır; değişkenler ve sunucudaki kayıt geçmişi korunur.</li><li>Günlük ve sonuç geçmişi de yeni ada taşınır.</li><li><b>macOS Keychain</b>'deki passphrase ve Knock kaydı otomatik taşınmaz; passphrase kullanıyorsanız yeniden eklemeniz gerekir.</li></ul>`,
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
    if (jobBusy()) return;
    let info;
    try { info = await json("/api/server-files?name=" + encodeURIComponent(name)); }
    catch (e) { toast("Okunamadı: " + e.message); return; }
    const files = info.files || [], hasKey = files.some(f => f.isKey), state = info.state || [];
    openDlg({
      title: "Sunucuyu sil", sub: name,
      body: `<div class="notice danger"><span class="ico" aria-hidden="true">⚠</span><div><b>Bu işlem geri alınamaz.</b> <span class="mono">servers/${esc(name)}/</span> klasörü ve içindeki her şey silinir${hasKey ? "; <b>özel SSH anahtarı dahil</b>" : ""}. Sunucunun kendisine dokunulmaz.</div></div>
        <div class="small muted sj-2173cd">Silinecek dosyalar</div>
        <div class="chips">${files.map(f => `<span class="chip${f.isKey ? " chip-key" : ""}">${esc(f.name)}</span>`).join("") || '<span class="small muted">boş klasör</span>'}</div>
        ${hasKey ? '<label class="small sj-6693ed"><input type="checkbox" id="dl-key"> <span><b>Özel anahtarın silineceğini anlıyorum</b>; sunucuya giriş için tek yol buysa erişimi kaybedebilirim.</span></label>' : ""}
        ${state.length ? `<label class="small sj-6abd0a"><input type="checkbox" id="dl-logs"> <span>Bu bilgisayardaki günlük ve sonuç geçmişini de sil (${state.map(esc).join(", ")}). İşaretlemezseniz günlükler kalır.</span></label>` : ""}
        <div class="field sj-d6f2af"><label for="dl-name">Onay için sunucu adını yazın: <span class="mono">${esc(name)}</span></label><input id="dl-name" autocomplete="off" spellcheck="false" placeholder="${esc(name)}"><div class="hint">Büyük/küçük harf duyarlıdır. Motor Keychain kaydını silmek isteyip istemediğinizi ayrıca sorabilir.</div></div>`,
      ok: "Sil", okClass: "dangerbtn",
      onOk: async () => {
        $("#dlg-ok").disabled = true;
        let r;
        try { r = await json("/api/server-remove", { name, confirm: $("#dl-name").value, ack: !!($("#dl-key") && $("#dl-key").checked) }); }
        catch (e) { $("#dlg-ok").disabled = false; $("#dlg-err").textContent = "✖ " + e.message; return; }
        delName = name; delPurge = !!($("#dl-logs") && $("#dl-logs").checked);
        closeDlg();
        openRun("", "", "remove"); jobs.setMode("remove");
        $("#run-title").innerHTML = `Sunucu siliniyor <span class="draft-tag sj-0996f7">${esc(name)}</span>`;
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
    if (jobBusy(server)) return;
    openRun(server, "", task);
    jobs.setMode("task");
    const [, title, text] = TASKS[task];
    $("#run-title").innerHTML = `${esc(title)} <span class=\"draft-tag sj-c8ceca\">${esc(server)}</span>`;
    $("#run-sub").textContent = "Bağlantı hazırlığı";
    $("#run-confirm-text").innerHTML = text;
    $("#run-confirm-note").textContent = "Motorun soruları çıktıda belirir: Evet/Hayır düğmeleriyle ya da kutuya yazarak yanıtlayın. Parolalar sistem parola penceresine yazılır.";
    $("#run-go").textContent = "Başlat";
  }
  function renderAddSteps() {
    const wrap = $("#add-steps");
    if (!addServer) { wrap.hidden = true; $("#add-form-wrap").hidden = false; return; }
    const sv = appState.home.servers.find(x => x.name === addServer);
    $("#add-form-wrap").hidden = true; wrap.hidden = false;
    const hasKey = !!(sv && sv.hasKey);
    const done = t => addDone.has(addServer + ":" + t);
    const step = (n, title, desc, st, btn) => `<div class="act sj-b4411e"><div class="badge ${st === "ok" ? "ok" : st === "todo" ? "warn" : "plain"} sj-03c4bb">${st === "ok" ? "✔" : n}</div><div><div class="sj-eed0f8">${title}</div><div class="small muted">${desc}</div></div><div>${btn || ""}</div></div>`;
    wrap.innerHTML = `<div class="notice ok sj-da12f2"><span class="ico" aria-hidden="true">✔</span><div><b>${esc(addServer)}</b> oluşturuldu. Sunucuyu kullanıma hazırlamak için adımları sırayla izleyin; her adım isteğe bağlıdır ve yeniden yapılabilir.</div></div>
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
      <div class="row-flex sj-d6f2af"><button class="btn primary" id="add-open">Sunucu sayfasına git</button><button class="btn ghost" id="add-another">Yeni bir sunucu daha ekle</button></div>`;
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
    if (!d.ok) { $("#add-preview").innerHTML = '<div class="empty-state sj-5417cf"><h3>Önizleme</h3><p>Alanlar geçerli olunca oluşturulacak dosya ve sonraki adımlar burada görünür.</p></div>'; return; }
    $("#add-preview").innerHTML = `
      <div class="small muted">Oluşturulacak dosya <span class="draft-tag">yazılmayacak</span></div>
      <div class="mono small sj-fe7b49">${esc(d.envPath)}</div>
      <div class="codeblock">${esc(d.envText)}</div>
      <div class="small muted">Anahtar</div>
      <div class="mono small sj-6926eb">${esc(d.keyPath)}</div>
      <div class="small muted">Sonraki adımlar (terminalde, parolayı ve passphrase'i orada siz girersiniz)</div>
      ${d.commands.map((c, i) => `<div class="codeblock" data-i="${i}">${esc(c)}</div>`).join("")}
      <div class="row-flex"><button class="btn sm" id="add-copy">Komutları kopyala</button><button class="btn sm primary" id="add-create">Sunucuyu oluştur</button></div>
      <div id="add-err" class="small sj-d64673" role="alert"></div>`;
    $("#add-create").addEventListener("click", async () => {
      $("#add-create").disabled = true; $("#add-err").textContent = "";
      try { await json("/api/server-create", { name: $("#add-name").value, host: $("#add-host").value, user: $("#add-user").value, port: $("#add-port").value, os: $("#add-os").value }); }
      catch (e) { $("#add-create").disabled = false; $("#add-err").textContent = "✖ " + e.message; return; }
      addServer = $("#add-name").value.trim(); toast("Sunucu oluşturuldu"); await load(); renderAddSteps();
    });
    $("#add-copy").addEventListener("click", async () => { try { await navigator.clipboard.writeText(d.commands.join("\n")); toast("Komutlar kopyalandı"); } catch { prompt("Kopyalayın:", d.commands.join("\n")); } });
  }

  /* ---------------- Paketler sayfası (js/ui/packages.js) ---------------- */
  const packages = initPackages({ openDlg, closeDlg, isDlgOpen: () => dlg.classList.contains("open"), jobs });

  /* ---------------- Lynis ve Günlükler sayfaları (js/ui/lynis.js, js/ui/logs.js) ---------------- */
  const lynis = initLynis({ showView: n => showView(n) });
  const logs = initLogs({ showView: n => showView(n) });

  /* ---------------- Sunucu sayfası ---------------- */
  const TAB_ORDER = ["linux-hardening", "utility", "kurulum"];

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
      (w.length > 2 ? `<details class="sj-96ad60"><summary class="small muted sj-3b6a3a">${w.length - 2} uyarı daha</summary><ul>${w.slice(2).map(x => `<li>${esc(x)}</li>`).join("")}</ul></details>` : "") : "";
    const cat = actCat(a);
    return `<div class="act r-${cat}" data-cat="${cat}"><div class="act-head"><span class="act-name">${esc(a.id)}</span>${effectBadges(a)}${stale}</div>
      ${warn}${keys ? `<div class="chips">${keys}</div>` : ""}
      <div class="act-foot"><span>${last}</span>${req.length ? `<span>· önkoşul: ${req.join(", ")}</span>` : ""}<span class="grow"></span>
        ${a.runnable ? `<button class="btn sm primary" data-run="${esc(pkg.id)}|${esc(a.id)}">Çalıştır</button>` : a.write ? (a.destructive ? `<button class="btn sm dangerbtn" data-write="${esc(pkg.id)}|${esc(a.id)}" title="YIKICI: geri alınamayabilir; plan, snapshot, ad yazma ve ek onaydan geçer">Yıkıcı eylem…</button>` : `<button class="btn sm warnbtn" data-write="${esc(pkg.id)}|${esc(a.id)}" title="Gerçek sunucuya yazar; plan, snapshot ve onay adımlarından geçer">Onayla ve çalıştır…</button>`) : a.terminal ? `<button class="btn sm" data-term="${esc(pkg.id)}|${esc(a.id)}" title="Sudo parolası gerekir; Terminal'de girersiniz">Terminal'de çalıştır</button>` : `<span class="small muted sj-45fcdd">${esc(a.reason || "")}</span>`}</div></div>`;
  }

  // paketin bu sunucudaki değişkenleri (salt okunur): ayarlı olanlar ve eylemlerin zorunlu kıldıkları
  function varsCard(p) {
    const vs = p.vars || [];
    const editBtn = `<span class="sj-97445a"></span><button class="btn sm" data-vars="${esc(p.id)}">Düzenle…</button>`;
    if (!vs.length) return `<div class="card"><div class="card-head"><h2>Değişkenler</h2><span class="hint">bu sunucuda ayarlı değişken yok</span>${editBtn}</div><div class="card-body small muted">Hiç değişken ayarlanmamış; paket varsayılanlarıyla çalışır. <b>Düzenle…</b> ile tüm değişkenleri görüp ayarlayabilirsiniz.</div></div>`;
    const row = v => {
      const shown = v.set ? v.value : v.default;
      const state = v.set ? '<span class="badge ok">ayarlı</span>' : v.needed ? '<span class="badge danger">eksik · gerekli</span>' : '<span class="badge plain">varsayılan</span>';
      const full = v.description || "", desc = full.length > 150 ? full.slice(0, 150).replace(/\s+\S*$/, "") + "…" : full;
      return `<div class="urow sj-ca7706"><div><div class="mono small sj-eed0f8">${esc(v.name)}</div><div>${state}${v.needed && v.set ? ' <span class="badge plain">gerekli</span>' : ""}</div></div>
        <div><span class="chip">${esc(shown) || "(boş)"}</span></div><div class="small muted" title="${esc(full)}">${esc(desc)}</div></div>`;
    };
    return `<div class="card"><div class="card-head"><h2>Değişkenler</h2><span class="hint">bu sunucu için · ${vs.length} ayarlı/gerekli</span>${editBtn}</div>
      <div class="card-body tight">${vs.map(row).join("")}</div>
      <div class="card-body small muted sj-2eea7e">Eylemler bu değişkenlerle çalışır. Tüm değişkenleri görmek ve değiştirmek için <b>Düzenle…</b>.</div></div>`;
  }

  /* ---------------- Sunucu özeti ---------------- */
  const COPY_ICON = '<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15V6a2 2 0 0 1 2-2h9"/></svg>';
  const GAUGE_PATH = "M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831";
  const gauge = score => { const v = Math.max(0, Math.min(100, score)), c = v >= 80 ? "ok" : v >= 60 ? "warn" : "danger"; return `<div class="gauge ${c}"><svg viewBox="0 0 36 36" aria-hidden="true"><path class="g-bg" stroke-width="3" fill="none" d="${GAUGE_PATH}"/><path class="g-fg" stroke-dasharray="${v}, 100" stroke-width="3.2" stroke-linecap="round" fill="none" d="${GAUGE_PATH}"/></svg><div class="g-num"><b>${v}</b><span>/100</span></div></div>`; };

  // Bağlantı hazırlığı satırı: st = ok | warn | todo | idle (ok dışında adım numarası görünür)
  const prepRow = (n, st, title, badge, desc, btn) => `<div class="prep-row"><div class="prep-dot ${st}" aria-hidden="true">${st === "ok" ? "✓" : st === "warn" ? "!" : n}</div><div class="prep-txt"><div class="prep-title">${title}${badge ? ` <span class="prep-badge ${st}">${badge}</span>` : ""}</div><div class="small muted">${desc}</div></div>${btn || ""}</div>`;

  function renderOzet(s, pk) {
    const k = (appState.srv && appState.srv.knock) || {}, pr = (appState.srv && appState.srv.prep) || {};
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
        <div class="oz-body">${kvr("Sistem", esc((OS_LABEL[s.os] || "Linux") + (s.platform ? " · " + s.platform : "")))}${kvr("SSH anahtarı", s.hasKey ? '<span class="badge ok">var</span>' : '<span class="badge warn">yok</span>')}${kvr("Port knocking", !k.enabled ? '<span class="muted">kapalı</span>' : k.hasSeq ? `açık · ${esc(k.proto)}` : '<span class="sj-81fa6c">açık · dizi yok</span>')}</div>
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

    $$("#srv-body [data-goto]").forEach(b => b.addEventListener("click", () => { appState.srvTab = b.dataset.goto; history.replaceState(null, "", `#s=${encodeURIComponent(appState.srvName)}&p=${encodeURIComponent(appState.srvTab)}`); renderServer(); }));
    $("#srv-body [data-go-logs]")?.addEventListener("click", () => goLogs(appState.srvName));
    $("#srv-body [data-go-lynis]")?.addEventListener("click", () => goLynis(appState.srvName, s.lynis && s.lynis.warnings > 0 ? "uyari" : "all"));
    $$("#srv-body [data-task]").forEach(b => b.addEventListener("click", () => (b.dataset.task === "knock" ? openKnock(appState.srvName) : openTask(appState.srvName, b.dataset.task))));
    $("#srv-body [data-test]")?.addEventListener("click", () => openRun(appState.srvName, "utility", "durum"));
  }

  /* ---------------- Süzgeçli geçişler ve bağlantı bilgisi ---------------- */
  // Günlükler sayfasını yalnızca bu sunucunun günlükleri süzülmüş olarak aç.
  const goLogs = server => logs.go(server);
  // Lynis sayfasını bu sunucu seçili ve (verilmişse) bulgu türü süzülmüş olarak aç.
  const goLynis = (server, kind) => lynis.go(server, kind);

  async function openEditAddr(name) {
    let cur;
    try { cur = (await json("/api/server?name=" + encodeURIComponent(name))).server; }
    catch (e) { toast("Okunamadı: " + e.message); return; }
    openDlg({
      title: "Bağlantı bilgilerini düzenle", sub: name,
      body: `<p class="small muted sj-249d01">Adres, kullanıcı ya da port değişince SSH anahtarı, Keychain parolası ve knock ayarları <b>aynen kalır</b> (sunucu adına bağlıdır). Adres değiştiyse sunucunun parmak izi ilk bağlantıda yeniden sorulabilir.</p>
        <div class="field"><label for="ea-host">Adres (IP ya da sunucu adı)</label><input id="ea-host" autocomplete="off" spellcheck="false" autocapitalize="off" value="${esc(cur.host || "")}"></div>
        <div class="row-flex sj-aba5aa">
          <div class="field sj-eb6218"><label for="ea-user">Kullanıcı</label><input id="ea-user" autocomplete="off" spellcheck="false" autocapitalize="off" value="${esc(cur.user || "")}"></div>
          <div class="field"><label for="ea-port">Port</label><input class="sj-607468" id="ea-port" type="number" min="1" max="65535" inputmode="numeric" value="${esc(cur.port || "22")}"></div></div>`,
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

  /* ---------------- Port knocking formu (js/ui/knock.js) ---------------- */
  async function reloadServer() {
    if (!appState.srvName) return;
    try { appState.srv = await json("/api/server?name=" + encodeURIComponent(appState.srvName)); if (appState.view === "server") renderServer(); } catch (e) { /* sonraki otomatik yenileme alır */ }
  }
  const knock = initKnock({ openDlg, closeDlg, reloadServer });
  const openKnock = knock.open;

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
    if (!l) return `<div class="card hero-card"><div class="hero-top"><div><div class="hero-kicker">Lynis skoru — ${esc(s.name)}</div><div class="small muted sj-fe7b49">Bu sunucu için ölçüm yok. Denetim çalıştırılınca skor ve geçmiş burada görünür.</div></div>${btn}</div></div>`;
    const st = auditStale(s);
    const h = l.history || [], prev = h.length > 1 ? h[h.length - 2] : null, d = prev === null ? 0 : l.score - prev;
    const prevTxt = prev === null ? "henüz yok" : `${prev}${d === 0 ? " (değişmedi)" : ` (${d > 0 ? "▲ +" : "▼ "}${d})`}`;
    return `<div class="card hero-card"><div class="hero-top">
        <div><div class="hero-kicker">Lynis skoru — ${esc(s.name)} <span class="hero-sep" aria-hidden="true"></span> <span class="muted">Son denetim: ${esc((l.date || "").slice(0, 10))}</span>${st ? ' <span class="stale-flag">⚠ güncel değil</span>' : ""}</div>
          <div class="hero-score"><b>${l.score}</b><span class="hero-of">/ 100</span><span class="badge ${l.warnings ? "warn" : "ok"}">${l.warnings} uyarı</span><span class="badge info">${l.suggestions} öneri</span></div></div>
        <div class="hero-side"><div class="hero-meta"><b>Toplam ${h.length} ölçüm</b><span class="muted">Önceki: ${prevTxt}</span></div>${btn}</div></div>
      ${st ? `<div class="notice sj-d870e8"><span class="ico" aria-hidden="true">⚠</span><div>Son denetimden sonra sertleştirme uygulandı (<span title="${esc(localTime(st.time))}">${esc(ago(st.time))} önce</span>). Skor ve bulgular eski olabilir; <b>Yeniden denetle</b> ile güncelleyin.</div></div>` : ""}
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
      <p class="small sj-c42b62">${esc(p.description)}</p>
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
    if (p.items && p.items.length) return p.items.map(it => ({ id: it.id, label: it.label || it.id, description: it.description || "", params: it.params || [], group: "", actions: it.actions || [], states: it.states || {} }));
    if (p.id === "kurulum") return LEGACY_ITEMS;
    return null;
  }

  async function loadCatVars(pkg) {
    catSrv = appState.srvName; catPkg = pkg; // başarısız olsa da yeniden denemeyi bir sonraki tazelemeye bırak (döngü olmasın)
    try { catVars = (await json(`/api/vars?server=${encodeURIComponent(appState.srvName)}&package=${encodeURIComponent(pkg)}`)).vars || []; } catch (e) { return; }
    if (appState.view === "server" && appState.srvTab === pkg && appState.srv) renderServer();
  }

  function catalogStore(s, p, ok, na, items) {
    const legacy = !(p.items && p.items.length);
    const d = p.actions.find(a => a.id === "durum"), k = (d && d.keys) || {}, known = "kurulu" in k;
    const inst = Object.fromEntries(splitPlus(k.kurulu)), ports = Object.fromEntries(splitPlus(k.portlar));
    const dattr = d ? actBtnAttr(d, p) : "";
    const onN = items.filter(it => it.id in inst).length;
    const loaded = catSrv === appState.srvName && catPkg === p.id;
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
      const off = (label, tip) => `<button class="btn sm" disabled title="${esc(tip)}">${label}</button>`;
      const tip = "Öğeye özel çalıştırma yakında: önizleme önkoşulu her öğe için ayrı tutulacak. Şimdilik aşağıdaki Genel eylemler kullanılır.";
      // öğe eylemi düğmesi: durum BU öğenin kapsamında hesaplanmıştır (önkoşul = aynı öğe + mod için taze önizleme)
      const ACT_LBL = { preview: "Önizle", kur: "Kur", yukselt: "Yükselt", kaldir: "Kaldır" };
      const ibtn = a => {
        const st = (it.states || {})[a], label = ACT_LBL[a];
        if (!st) return off(label, "durum okunamadı");
        const kind = st.runnable ? "irun" : st.write ? "iwrite" : st.terminal ? "iterm" : "";
        if (!kind) return off(label, st.reason || "şimdi çalıştırılamaz");
        const cls = st.destructive ? "dangerbtn" : st.write ? "warnbtn" : "";
        return `<button class="btn sm ${cls}" data-${kind}="${esc(p.id)}|${esc(a)}|${esc(id)}" title="${esc(st.reason || "")}">${label}</button>`;
      };
      const itemActs = Object.keys(ACT_LBL).filter(a => (it.actions || []).includes(a));
      const live = itemActs.length > 0; // öğe eylemlerini bildirmeyen (ör. eski kurulum 0.1.0) kartlar eskisi gibi kapalı kalır
      const btns = live ? itemActs.filter(a => a === "preview" || (a === "kur" ? !on : on)).map(ibtn).join("") : `${off("Önizle", tip)}${on ? off("Yükselt", tip) + off("Kaldır", tip) : off("Kur", tip)}`;
      const ico = esc(LEGACY_ICONS[id] || String(it.label).charAt(0).toUpperCase());
      return `<div class="card app-card" data-app="${esc(id)}" data-st="${!known ? "?" : on ? "on" : "off"}">
        <div class="app-top"><div class="app-ico" aria-hidden="true">${ico}</div><div class="app-titles"><div class="app-title"><span>${esc(it.label)}</span>${badge}</div><p class="app-desc">${esc(it.description)}</p></div></div>
        ${meta.length ? `<div class="app-meta">${meta.map(m => `<span class="meta-chip">${m}</span>`).join("")}</div>` : ""}
        <div class="app-vars"><div class="av-head"><span class="oz-title">Değişkenler</span><button class="link" data-kvars="${esc(groups.length === 1 ? groups[0] : "")}">Düzenle…</button></div>${rows}</div>
        <div class="app-foot"><span class="small muted">${live ? "Önizlemeden sonra kurulur" : "Eylemler yakında"}</span><div class="app-btns">${btns}</div></div></div>`;
    }).join("");
    const chip = (key, label, n) => `<button class="fchip" data-sf="${key}" aria-pressed="${storeFilter === key}">${label}${n === undefined ? "" : ` <span class="muted">${n}</span>`}</button>`;
    // genel ayarlar: eski kurulum için sabit dört değişken; diğerlerinde hiçbir öğeye ait olmayan değişkenler (en çok 8)
    const genPairs = legacy && p.id === "kurulum" ? LEGACY_GEN.map(([l, n]) => [l, n]) : catVars.filter(v => !claimed.has(v.name)).slice(0, 8).map(v => [v.name.startsWith(p.id + "_") ? v.name.slice(p.id.length + 1) : v.name, v.name]);
    const gen = genPairs.map(([l, n]) => `<div class="kv"><span class="muted">${esc(l)}</span><span class="chip" title="${esc(n)}">${esc(catVal(n)) || "—"}</span></div>`).join("") || '<div class="small muted">Ortak ayar yok.</div>';
    const itemVar = p.itemVar || (legacy && p.id === "kurulum" ? "kurulum_yazilim" : "");
    const sub = legacy ? "Kurulum motoru" : esc(p.name || p.id);
    return `<div class="store-head"><div><h2>Uygulama kataloğu</h2><div class="small muted">${sub} <span class="mono">v${esc(p.version)}</span> · bu USB'de.${legacy ? " Docker ya da yerel paket (native) olarak kurulur." : p.description ? " " + esc(p.description) : ""}</div></div>
        <div class="fchips2">${chip("all", "Tümü", items.length)}${known ? chip("on", "Kurulu", onN) + chip("off", "Kurulabilir", items.length - onN) : ""}</div></div>
      ${known ? "" : `<div class="notice sj-2b583d"><span class="ico" aria-hidden="true">ℹ</span><div>Hangi uygulamaların kurulu olduğu henüz okunmadı. ${dattr ? `<button class="link" ${dattr}>Durumu oku</button> (salt okunur).` : ""}</div></div>`}
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
    $$("#srv-body [data-kvars]").forEach(b => b.addEventListener("click", () => openVars(appState.srvName, pkg, b.dataset.kvars || undefined)));
    const itemRun = (attr, fn) => $$(`#srv-body [data-${attr}]`).forEach(b => b.addEventListener("click", () => { const [pid, aid, iid] = b.dataset[attr].split("|"); fn(appState.srvName, pid, aid, iid); }));
    itemRun("irun", openRun); itemRun("iwrite", openWrite); itemRun("iterm", openTerm);
    $("#srv-body [data-kvars-all]")?.addEventListener("click", () => openVars(appState.srvName, pkg));
    $$("#srv-body [data-actcat]").forEach(b => b.addEventListener("click", () => { actFilter = b.dataset.actcat; applyActFilter(); }));
    if (!$$("#srv-body [data-sf]").some(b => b.dataset.sf === storeFilter)) storeFilter = "all";
    applyStoreFilter(); applyActFilter();
    if (catSrv !== appState.srvName || catPkg !== pkg) loadCatVars(pkg);
  }

  function renderServer() {
    if (!appState.srv) return;
    const s = appState.srv.server;
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
    const pk = [...appState.srv.packages].sort((a, b) => ti(a.id) - ti(b.id));
    const tabs = [{ id: "ozet" }, ...pk];
    if (!tabs.some(p => p.id === appState.srvTab)) appState.srvTab = "ozet";
    $("#srv-tabs").innerHTML = tabs.map(p => `<button role="tab" id="tab-${esc(p.id)}" aria-selected="${p.id === appState.srvTab}" data-p="${esc(p.id)}">${esc(p.id === "ozet" ? "Özet" : TAB_LABEL[p.id] || p.name || p.id)}</button>`).join("");
    $$("#srv-tabs button").forEach(b => b.addEventListener("click", () => { appState.srvTab = b.dataset.p; history.replaceState(null, "", `#s=${encodeURIComponent(appState.srvName)}&p=${encodeURIComponent(appState.srvTab)}`); renderServer(); }));

    const p = pk.find(x => x.id === appState.srvTab);
    if (appState.srvTab === "ozet") { renderOzet(s, pk); return; }
    setTimeout(() => $$("#srv-body [data-vars]").forEach(b => b.addEventListener("click", () => openVars(appState.srvName, b.dataset.vars))), 0);
    setTimeout(() => $$("#srv-body [data-write]").forEach(b => b.addEventListener("click", () => { const [pid, aid] = b.dataset.write.split("|"); openWrite(appState.srvName, pid, aid); })), 0);
    setTimeout(() => $$("#srv-body [data-term]").forEach(b => b.addEventListener("click", () => { const [pid, aid] = b.dataset.term.split("|"); openTerm(appState.srvName, pid, aid); })), 0);
    setTimeout(() => $$("#srv-body [data-run]").forEach(b => b.addEventListener("click", () => { const [pid, aid] = b.dataset.run.split("|"); openRun(appState.srvName, pid, aid); })), 0);
    if (!p) { $("#srv-body").innerHTML = '<div class="card"><div class="card-body small muted">Bu sunucu için paket bulunamadı.</div></div>'; return; }
    const ok = p.actions.filter(a => a.applicable), na = p.actions.filter(a => !a.applicable);
    const cat = catalogItems(p);
    if (cat) { $("#srv-body").innerHTML = catalogStore(s, p, ok, na, cat); bindCatalog(p.id); return; }
    $("#srv-body").innerHTML = `${p.id === "linux-hardening" ? lynisHero(s, p) : ""}<div class="pk-top">${pkgInfoCard(p, ok)}${varsCard(p)}</div>${statusCard(p)}${actionsSection(ok, na, p)}`;
    $$("#srv-body [data-actcat]").forEach(b => b.addEventListener("click", () => { actFilter = b.dataset.actcat; applyActFilter(); }));
    $("#srv-body [data-go-lynis-p]")?.addEventListener("click", () => goLynis(appState.srvName, s.lynis && s.lynis.warnings > 0 ? "uyari" : "all"));
    applyActFilter();
  }

  async function act(name, action, msg) {
    try { const r = await json("/api/action", { name, action }); toast(r && r.knock ? "Terminal açıldı · 🚪 knock Terminal'de gönderiliyor" : msg); }
    catch (e) { toast("Hata: " + e.message); }
  }

  async function openServer(name, tab) {
    appState.srvName = name; appState.srvTab = tab || "ozet";
    appState.srv = null;
    showView("server"); renderSrvTree();
    setHash(`#s=${encodeURIComponent(name)}&p=${encodeURIComponent(appState.srvTab || "")}`, new URLSearchParams(location.hash.slice(1)).get("s") === name);
    $("#srv-head").innerHTML = `<button class="back" id="srv-back">← Sunucular</button><div class="srv-title"><h1>${esc(name)}</h1></div>`;
    $("#srv-back").addEventListener("click", () => showView("servers"));
    $("#srv-tabs").innerHTML = ""; $("#srv-body").innerHTML = '<div class="small muted">yükleniyor…</div>';
    // Yanıt gelene kadar kullanıcı başka sunucuya/sayfaya geçmiş olabilir: eski yanıt yenisini ezmesin.
    let data;
    try { data = await json("/api/server?name=" + encodeURIComponent(name)); }
    catch (e) { if (appState.srvName === name && appState.view === "server") $("#srv-body").innerHTML = `<div class="notice danger"><span class="ico" aria-hidden="true">✕</span><div>Sunucu açılamadı: ${esc(e.message)}</div></div>`; return; }
    if (appState.srvName !== name || appState.view !== "server") return;
    appState.srv = data;
    renderServer();
  }

  $$(".mode-toggle button").forEach(b => b.addEventListener("click", () => {
    appState.mode = b.dataset.mode;
    try { localStorage.setItem("sshpocket.mode", appState.mode); } catch (e) { /* önemli değil */ }
    render();
  }));

  /* ---------------- Arama ---------------- */
  const search = $("#search-input");
  search.addEventListener("input", () => { appState.query = search.value.trim().toLowerCase(); render(); });
  document.addEventListener("keydown", e => {
    if (e.key === "/" && document.activeElement !== search && !/input|textarea/i.test(document.activeElement.tagName)) { e.preventDefault(); search.focus(); }
  });

  /* ---------------- Bağlam menüsü ---------------- */
  const { open: openShortcuts, close: closeShortcuts } = initShortcuts();
  const sshImport = initSshImport({ load: () => load() });
  const ctxMenu = initContextMenu({
    // menü düğmesi seçilince (menü zaten kapanmıştır): eylem app.js'tedir
    onPick: async (a, name) => {
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
    },
  });
  const openCtx = ctxMenu.open, closeCtx = ctxMenu.close;
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
  document.addEventListener("keydown", e => { if (e.key === "Escape") { closeCtx(); closeWizard(); closeRun(); closeVars(); closeShortcuts(); } });

  /* ---------------- Sunucu karşılaştırma (js/compare.js) ---------------- */
  window.KeyLabCompare.init({
    getHome: () => appState.home, json, toast, ago, osIcon, dotState, stateText,
    tabLabel: id => TAB_LABEL[id] || (appState.home.packages.find(p => p.id === id) || {}).name || id,
    openServer, openVars, goLynis,
  });

  window.addEventListener("keylab:toast", e => toast(String(e.detail || "")));
  window.KeyLabBackup.init({ json, toast, reload: load });
  window.KeyLabOnboarding.init({
    goAdd: () => showView("add"),
    openImport: () => { showView("add"); sshImport.open(); },
    goSettings: () => showView("settings"),
  });
  window.KeyLabDock.init({
    getHome: () => appState.home, json, reload: load, ago, localTime, openLastLog: (p, sv) => logs.openLast(p, sv),
    tabLabel: id => TAB_LABEL[id] || (appState.home.packages.find(p => p.id === id) || {}).name || id,
    // çalışan işin penceresini aç; toplu işlem sürüyorsa onun penceresini
    openJob: async id => {
      if (($("#batch-open").textContent || "").startsWith("⏳")) { $("#batch-open").click(); return; }
      await openJob(id);
    },
  });
  window.KeyLabBatch.init({ json, toast, reload: load, targets: () => visibleServers.slice(), tabLabel: id => TAB_LABEL[id] || (appState.home.packages.find(p => p.id === id) || {}).name || id });
  adoptJob();
  window.KeyLabPalette.init({ getHome: () => appState.home, showView, openServer, goLynis, goLogs, openShortcuts, reload: load });
  window.KeyLabPrivacy.init();
  window.KeyLabReports.init({
    getHome: () => appState.home, json, toast, stateText,
    tabLabel: id => TAB_LABEL[id] || (appState.home.packages.find(p => p.id === id) || {}).name || id,
  });

  $("#refresh-all").addEventListener("click", async () => { const b = $("#refresh-all"); b.disabled = true; await load(); b.disabled = false; toast("Yenilendi"); });

  /* ---------------- Sihirbaz (js/ui/wizard.js) ---------------- */
  const { open: openWizard, close: closeWizard, isOpen: isWizardOpen } = initWizard({ getHome: () => appState.home });

  let rz = 0;
  window.addEventListener("resize", () => {
    clearTimeout(rz);
    rz = setTimeout(() => { if (appState.view === "lynis" && lynis.repaint()); else if (appState.view === "server" && appState.srv) renderServer(); }, 250);
  });

  /* ---------------- Başlangıç ---------------- */
  json("/api/about").then(a => { appState.askAvail = !!a.askpass; paintAsk(); }).catch(() => {});
  const startHash = new URLSearchParams(location.hash.slice(1)); // showView adres çubuğundaki #s=… kısmını siler: ÖNCE oku
  appState.routing = true; showView(VIEWS.includes(startHash.get("v")) ? startHash.get("v") : "dash"); appState.routing = false;
  load().then(() => {
    if (startHash.get("s")) { appState.routing = true; openServer(startHash.get("s"), startHash.get("p") || ""); appState.routing = false; }
    // Lynis sayfası sunucu listesine (home) bağlıdır: adresle/yenilemeyle doğrudan açıldıysa liste YÜKLENMEDEN çizilmişti ("Sunucu yok"); şimdi yeniden çiz
    else if (appState.view === "lynis") lynis.load();
  });
  setInterval(() => {
    if (document.hidden || isWizardOpen()) return;
    load();
    if (appState.view === "server" && appState.srvName) json("/api/server?name=" + encodeURIComponent(appState.srvName)).then(r => { appState.srv = r; renderServer(); }).catch(() => {});
  }, 10000);
})();
