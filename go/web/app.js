import { $, $$, t, esc, api, json, toast, ago, localTime, isOk, resHtml } from "./js/lib/util.js";
import { initShortcuts } from "./js/ui/shortcuts.js";
import { initContextMenu } from "./js/ui/context-menu.js";
import { initSshImport } from "./js/ui/ssh-import.js";
import { initWizard } from "./js/ui/wizard.js";
import { initJobs } from "./js/ui/jobs.js";
import { initLynis, chart } from "./js/ui/lynis.js";
import { initLogs } from "./js/ui/logs.js";
import { initPackages } from "./js/ui/packages.js";
import { initVars } from "./js/ui/vars.js";
import { initServer } from "./js/ui/server.js";
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

  /* ---------------- Değişken düzenleme (js/ui/vars.js) ---------------- */
  const { open: openVars, close: closeVars, cur: varCur } = initVars({ renderServer: () => renderServer(), resetCatalog: () => serverPage.resetCatalog() });

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

  /* ---------------- Sunucu sayfası, Özet, paket sekmeleri ve katalog (js/ui/server.js) ---------------- */
  const serverPage = initServer({
    openRun, openWrite, openTerm, openVars, varCur, openDlg, closeDlg, showView: n => showView(n), load: () => load(),
    openTask: (s, k) => openTask(s, k), openCtx: (x, y, n) => openCtx(x, y, n), renderSrvTree: () => renderSrvTree(), setHash: (h, r) => setHash(h, r), stateText: s => stateText(s), logs, lynis,
  });
  const { openServer, renderServer, goLogs, goLynis, openEditAddr, act } = serverPage;

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
