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
import { initSettings } from "./js/ui/settings.js";
import { initDialog } from "./js/ui/dialog.js";
import { initQuit } from "./js/ui/quit.js";
import { initServerOps } from "./js/ui/serverops.js";
import { initAddServer } from "./js/ui/add.js";
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
    if (name === "add") initAddPage();
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

  // Pano durum çipi: tek satırlık özet (durum noktası, ad, son işlem); ayrıntı Sunucular sayfasındaki kartta
  function chip(s) {
    const st = dotState(s);
    const last = !s.valid ? "⚠ server.env geçersiz" : s.last ? `${isOk(s.last.result) ? "✔" : "✖"} ${esc(s.last.action)} · ${esc(ago(s.last.time))}` : "kayıt yok";
    return `<li class="schip ${st === "off" ? "is-off" : ""}" tabindex="0" data-name="${esc(s.name)}" data-state="${st}" aria-label="${esc(s.name)}, ${stateText(s)}" title="${esc(stateText(s))}">
      ${osIcon(s.os, 28)}<div class="schip-body"><div class="schip-name">${esc(s.name)}</div><div class="schip-sub ${!s.valid || (s.last && !isOk(s.last.result)) ? "bad" : ""}">${last}</div></div>
      <span class="status-dot ${st}" aria-label="${stateText(s)}"></span></li>`;
  }
  const DASH_MAX = 12; // Pano genel özettir: sorunlu sunucular önce, en çok 12 çip; kalanı Sunucular sayfasında

  // Sunucular sayfası kartı: Pano'daki küçük karoya göre adres, anahtar durumu, durum rozeti ve hızlı düğmeler de taşır
  function richTile(s) {
    const st = dotState(s), ok = st === "on", tone = ok ? "ok" : st === "unk" ? "plain" : st === "off" ? "danger" : "warn";
    const last = !s.valid ? "⚠ server.env geçersiz (IP, USER, PORT)" : s.last ? `${isOk(s.last.result) ? "✔" : "✖"} ${esc(s.last.package)} · ${esc(s.last.action)} · ${esc(ago(s.last.time))}` : "kayıt yok";
    const addr = s.valid ? `${s.user}@${s.host}:${s.port}` : "server.env geçersiz";
    const dis = s.valid ? "" : " disabled";
    return `<li class="stile stile-rich ${st === "off" ? "is-off" : ""}" tabindex="0" data-name="${esc(s.name)}" data-state="${st}" aria-label="${esc(s.name)}, ${stateText(s)}">
      <div class="stile-top">${osIcon(s.os, 40)}<div class="stile-ident"><div class="stile-name">${esc(s.name)}</div><div class="stile-os">${esc(OS_LABEL[s.os] || "Linux")}${s.platform ? ` · ${esc(s.platform)}` : ""}</div></div>
        <span class="stile-pill ${tone}"><span class="status-dot ${st}" aria-hidden="true"></span>${stateText(s)}</span>
        <button class="btn-icon" data-act="menu" aria-label="Daha fazla eylem"><svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><circle cx="5" cy="12" r="1.8"/><circle cx="12" cy="12" r="1.8"/><circle cx="19" cy="12" r="1.8"/></svg></button></div>
      <div class="stile-addr"><span class="mono">${esc(addr)}</span>${s.hasKey ? '<span class="badge ok">Anahtar var</span>' : '<span class="badge warn">Anahtar yok</span>'}</div>
      <div class="stile-last ${!s.valid || (s.last && !isOk(s.last.result)) ? "bad" : ""}">${last}</div>
      <div class="stile-foot"><button class="btn sm" data-qa="term"${dis}>Terminal</button><button class="btn sm" data-qa="copy"${dis}>Kopya</button><span class="grow"></span><button class="link" data-qa="open">Yönet →</button></div></li>`;
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
    const rowsWith = fn => (sv.map(appState.mode === "tiles" ? fn : serverRow).join("") || emptyServers) + (appState.mode === "tiles" && !appState.query && fleetFilter === "all" ? addTile : "");
    $("#servers-all").innerHTML = rowsWith(richTile); $("#servers-all").classList.toggle("tiles", appState.mode === "tiles");
    // Pano: arama uygulanır, Sunucular sayfasının süzgeci/görünüm kipi uygulanmaz; sorunlu sunucular (çevrimiçi olmayanlar) önce
    const dash = appState.home.servers.filter(s => matches(s.name, s.platform, s.last && s.last.package)).sort((x, y) => (dotState(x) === "on") - (dotState(y) === "on"));
    const shown = dash.slice(0, DASH_MAX), more = dash.length - shown.length;
    $("#servers").innerHTML = (shown.map(chip).join("") || emptyServers) + (more > 0 ? `<li class="schip more" tabindex="0" data-more="1">+ ${more} sunucu daha · Tümünü gör</li>` : "");
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
    renderAddSteps: () => renderAddSteps(), markTaskDone: k => add.markTaskDone(k), getRemoval: () => ops.getRemoval(), openLastLog: (p, sv) => logs.openLast(p, sv),
  });
  const { openRun, openWrite, openTerm, jobBusy, watchJob, closeRun, openJob, adoptJob } = jobs;

  /* ---------------- Tema ve Ayarlar (js/ui/settings.js) ---------------- */
  const settings = initSettings({ render: () => render() });
  const { load: loadSettings, paintAsk } = settings;

  /* ---------------- Ortak iletişim penceresi (js/ui/dialog.js) + yeniden adlandırma/silme (js/ui/serverops.js) ---------------- */
  const { dlg, openDlg, closeDlg } = initDialog();
  initQuit({ openDlg, closeDlg });
  const ops = initServerOps({ openDlg, closeDlg, jobs, load: () => load(), openServer: n => openServer(n) });
  const { openRename, openDelete } = ops;

  /* ---------------- Sunucu ekle (js/ui/add.js) ---------------- */
  const add = initAddServer({ load: () => load(), openServer: (n, t) => openServer(n, t), openVars: (n, p) => openVars(n, p), jobs });
  const { init: initAddPage, renderSteps: renderAddSteps, openTask } = add;

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
  // sunucu eylemleri: bağlam menüsü ve Sunucular sayfasındaki kart düğmeleri (Terminal, Kopya, Yönet) aynı işleve gider
  const pickServer = async (a, name) => {
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
      else if (a === "open") openServer(name);
    } catch (err) { toast("Hata: " + err.message); }
  };
  const ctxMenu = initContextMenu({
    // menü düğmesi seçilince (menü zaten kapanmıştır): eylem app.js'tedir
    onPick: pickServer,
  });
  const openCtx = ctxMenu.open, closeCtx = ctxMenu.close;
  function bindServers() {
    $$(".stile[data-add]").forEach(el => {
      el.addEventListener("click", () => showView("add"));
      el.addEventListener("keydown", e => { if (e.key === "Enter") showView("add"); });
    });
    $$(".schip[data-more]").forEach(el => {
      el.addEventListener("click", () => showView("servers"));
      el.addEventListener("keydown", e => { if (e.key === "Enter") showView("servers"); });
    });
    $$(".server[data-name], .stile[data-name], .schip[data-name]").forEach(el => {
      el.addEventListener("click", () => openServer(el.dataset.name));
      el.addEventListener("keydown", e => { if (e.key === "Enter" && e.target === el) openServer(el.dataset.name); }); // iç düğmelerde Enter yalnız o düğmeyi çalıştırır
      el.addEventListener("contextmenu", e => { e.preventDefault(); openCtx(e.clientX, e.clientY, el.dataset.name); });
      el.addEventListener("keydown", e => {
        if (e.key === "F10" || e.key === "ContextMenu") { e.preventDefault(); const r = el.getBoundingClientRect(); openCtx(r.left + 40, r.bottom + 4, el.dataset.name); }
      });
      $$("[data-qa]", el).forEach(qb => qb.addEventListener("click", e => { e.stopPropagation(); pickServer(qb.dataset.qa, el.dataset.name); }));
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
