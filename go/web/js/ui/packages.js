/* KeyLab — Paketler sayfası: paket listesi, ayrıntı penceresi, imzalı indeks kartı, "Güncellemeler" ve "Paketleri keşfet" pencereleri,
   yükseltme/kurma onayı girişi. app.js'ten çıkarıldı (refaktör 5b), kod BİREBİR; dış bağlantılar hooks ile. Testler: go/webtest/pkgpage.test.js */
import { $, $$, esc, json, ago, localTime } from "../lib/util.js";
import { appState } from "../state.js";
import { osIcon } from "../lib/icons.js";

// hooks: { openDlg(opts), closeDlg(), isDlgOpen() — ortak iletişim penceresi; jobs — iş izleyici API'si (openRun, jobBusy, setMode, setUpgradeTo) }
//   →  { load(), checkUpdates() }
export function initPackages(hooks) {
  const { openDlg, closeDlg, isDlgOpen, jobs } = hooks;
  const { openRun, jobBusy } = jobs;
  async function loadPackages() {
    let d;
    try { d = await json("/api/packages"); }
    catch (e) { $("#pkg-page").innerHTML = `<div class="notice danger"><span class="ico" aria-hidden="true">✕</span><div>Yüklenemedi: ${esc(e.message)}</div></div>`; return; }
    pkgRepo = Object.fromEntries((d.packages || []).map(p => [p.id, p.repo])); pkgInfo = Object.fromEntries((d.packages || []).map(p => [p.id, p]));
    const cmp = (a, b) => { const x = String(a).split(".").map(Number), y = String(b).split(".").map(Number); for (let i = 0; i < Math.max(x.length, y.length); i++) { const p = x[i] || 0, q = y[i] || 0; if (p !== q) return p < q ? -1 : 1; } return 0; };
    d.packages = d.packages || []; d.servers = d.servers || [];
    if (!d.packages.length) { $("#pkg-page").innerHTML = '<div class="card"><div class="card-body"><div class="empty-state"><h3>Paket bulunamadı</h3><p>Henüz kurulu paket yok. Paketler imzalı indeksten kurulur: indirilen her paketin imzası ve özeti doğrulanır.</p><button class="btn primary" id="pkg-empty-disc" type="button">Paketleri keşfet ve kur</button></div></div></div>'; $("#pkg-empty-disc").addEventListener("click", openDiscover); return; }
    const order = ["linux-hardening", "utility", "kurulum"];
    d.packages.sort((a, b) => (order.indexOf(a.id) + 1 || 99) - (order.indexOf(b.id) + 1 || 99));
    $("#pkg-page").innerHTML = `<div id="upd-card"></div><div class="notice sj-87c136"><span class="ico" aria-hidden="true">ℹ</span><div>Bir pakete tıklayın: sürümler, kilit tabanı ve sunuculardaki son kullanım açılır. "Kullanıldı", paketin o sunucuda en son çalıştırıldığı anlamına gelir; paketler sunucuya kurulmaz, kontrolcüden çalıştırılır.</div></div>` +
      `<div class="card pk-list">` + (d.packages.filter(p => !appState.query || `${p.id} ${p.name} ${p.repo || ""}`.toLowerCase().includes(appState.query)).map(p => {
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
      body: `<p class="small sj-75ff53">${esc(p.description || "")}</p>
        <div class="spec-grid">
          ${box("USB sürüm havuzu", (p.versions || []).map(v => `<span class="vchip${v === p.current ? " cur" : ""}">${esc(v)}</span>`).join("") || "—")}
          ${box("Kilit tabanı", `${p.floor ? `<span class="vchip">${esc(p.floor)}</span>` : '<span class="muted small">kilitte yok</span>'}${floorBad ? '<span class="badge danger">USB sürümü tabanın altında</span>' : p.floor ? '<span class="small muted sj-7e3a94">bu sürümün altı çalıştırılmaz</span>' : ""}`)}
          ${box("Sunucularda", `${sm.used} kullandı${sm.stale ? ` · <span class="sj-81fa6c">${sm.stale} eski</span>` : ""}${sm.none ? ` · <span class="muted sj-7e3a94">${sm.none} kullanmadı</span>` : ""}`)}
        </div>
        <div class="pk-fleet-head"><b>Sunucularda son kullanım</b><span class="small muted">paket sunucuya kurulmaz; bu, en son çalıştırıldığı sürümdür</span></div>
        ${many || sm.stale ? `<div class="row-flex sj-6ba842">
          ${many ? '<input class="sj-27af89" id="pd-q" type="search" placeholder="sunucu ara…" aria-label="Sunucu ara">' : ""}
          <label class="small sj-25b528"><input type="checkbox" id="pd-stale"${many && sm.stale ? " checked" : ""}> yalnız eski sürümlüler</label>
          ${sm.none ? `<label class="small sj-25b528"><input type="checkbox" id="pd-none"${many ? "" : " checked"}> kullanılmayanları da göster (${sm.none})</label>` : ""}</div>` : ""}
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
  const isUpdDlg = () => isDlgOpen() && $("#dlg-title").textContent === "Güncellemeler";
  function renderUpdates() {
    const el = $("#upd-card");
    if (!el) { if (isUpdDlg()) paintUpdDlg(); else if (isDiscDlg()) paintDisc(); return; } // kart yok (ör. paket yok boş durumu): açık pencere yine de güncellenir
    let st;
    if (updBusy) st = '<span class="spinner" aria-hidden="true"></span> İmzalı indeks alınıyor ve doğrulanıyor…';
    else if (updErr) st = '<span class="sj-497726">✕ denetlenemedi</span>';
    else if (!updRep) st = '<span class="muted">henüz denetlenmedi · hiçbir şey kurmaz</span>';
    else if (!updRep.ok) st = '<span class="sj-497726">✕ indeks doğrulanamadı (güvenli kapanma)</span>';
    else st = updUp() ? `<b class="sj-81fa6c">⬆ ${updUp()} güncelleme var</b>` : '<span class="sj-0b3aae">✔ hepsi güncel</span>';
    el.innerHTML = `<div class="card"><div class="card-head"><h2>Güncellemeler</h2><span class="hint">imzalı indeks · syhgorath/sshpocket-index</span><span class="sj-97445a"></span><span class="small">${st}</span><button class="btn sm" id="disc-open">Paketleri keşfet</button><button class="btn sm primary" id="upd-check"${updBusy ? " disabled" : ""}>${updRep || updErr ? "Ayrıntı" : "Güncellemeleri denetle"}</button></div></div>`;
    $("#upd-check").addEventListener("click", openUpdates);
    $("#disc-open").addEventListener("click", openDiscover);
    $$("#pkg-page [data-upd]").forEach(sp => { const r = updRep && updRep.ok ? updRep.rows.find(x => x.id === sp.dataset.upd) : null; sp.innerHTML = r && (r.verdict === "upgrade" || r.verdict === "install") ? `<span class="badge info">⬆ ${esc(r.version)} çıktı</span>` : ""; });
    if (isUpdDlg()) paintUpdDlg(); else if (isDiscDlg()) paintDisc();
  }
  function updBodyHtml() {
    const head = `<div class="row-flex sj-e4a423"><span class="small muted">Motorun <span class="mono">--components update</span> raporu: imza ve süre doğrulanır, hiçbir şey kurulmaz.</span><span class="sj-97445a"></span><button class="btn sm" id="upd-recheck"${updBusy ? " disabled" : ""}>Yeniden denetle</button></div>`;
    if (updBusy) return head + '<div class="small muted"><span class="spinner" aria-hidden="true"></span> İmzalı indeks alınıyor ve doğrulanıyor…</div>';
    if (updErr) return head + `<div class="notice danger"><span class="ico" aria-hidden="true">✕</span><div>${esc(updErr)}</div></div>`;
    if (!updRep) return head;
    if (!updRep.ok) return head + `<div class="notice danger"><span class="ico" aria-hidden="true">✕</span><div><b>İndeks doğrulanamadı</b> (güvenli kapanma): yükseltme yapılmaz.</div></div><details><summary class="small muted sj-3b6a3a">motor çıktısı</summary><pre class="mono small sj-a548ea">${esc(updRep.raw)}</pre></details>`;
    const rows = updRep.rows.map(r => { const [c, t] = VERDICT[r.verdict] || VERDICT.unknown; return `<div class="pkg-row"><div><div class="pkg-name">${esc(r.id)}</div><div class="pkg-meta"><span class="ver">${esc(r.installed === "-" ? "kurulu değil" : r.installed)}</span>${r.version ? `<span class="ver arrow">${esc(r.version)}</span>` : ""}<span class="badge ${c}">${t}</span></div></div>
      <div class="pkg-actions">${r.verdict === "upgrade" || r.verdict === "install" ? `<button class="btn sm primary" data-upg="${esc(r.id)}" data-from="${esc(r.installed)}" data-to="${esc(r.version)}">${r.verdict === "install" ? "Kur" : "Yükselt"}…</button>` : ""}</div></div>`; }).join("");
    return head + `<div class="small muted">İndeks seri <span class="mono">${esc(updRep.serial)}</span> · geçerlilik <span class="mono">${esc(updRep.valid)}</span> · ${updUp() ? `<b class="sj-81fa6c">${updUp()} güncelleme var</b>` : "hepsi güncel"}${updAt ? ` · denetim ${new Date(updAt).toLocaleTimeString("tr-TR")}` : ""}</div>
      <div>${rows}</div><details><summary class="small muted sj-3b6a3a">motorun ham raporu</summary><pre class="mono small sj-c2ee2b">${esc(updRep.raw)}</pre></details>`;
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
  const isDiscDlg = () => isDlgOpen() && $("#dlg-title").textContent === "Paketleri keşfet";
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
    }).join("") || '<div class="small muted sj-215683">Bu süzgeçle eşleşen paket yok.</div>';
    $$("#disc-list [data-dget]").forEach(b => b.addEventListener("click", () => { const { dget, from, to } = b.dataset; closeDlg(); openUpgrade(dget, from, to); }));
  }
  function paintDisc() {
    if (updBusy) { $("#dlg-body").innerHTML = '<div class="small muted"><span class="spinner" aria-hidden="true"></span> İmzalı indeks alınıyor ve doğrulanıyor…</div>'; return; }
    if (updErr || !updRep || !updRep.ok) { $("#dlg-body").innerHTML = `<div class="notice danger"><span class="ico" aria-hidden="true">✕</span><div>${updErr ? esc(updErr) : "İndeks doğrulanamadı (güvenli kapanma): paket listesi gösterilmez."}</div></div><div><button class="btn sm" id="disc-retry">Yeniden dene</button></div>`; $("#disc-retry").addEventListener("click", checkUpdatesUI); return; }
    const cnt = k => updRep.rows.filter(r => k === "all" || (k === "installed" ? r.installed !== "-" : r.verdict === k)).length;
    $("#dlg-body").innerHTML = `<div class="row-flex sj-e4a423"><input class="sj-a5d7a0" id="disc-q" type="search" placeholder="paket ara…" aria-label="Paket ara" value="${esc(discQ)}"><div class="seg" role="tablist" aria-label="Süzgeç">${DISC_F.map(([k, l]) => `<button class="btn sm${discF === k ? " primary" : ""}" data-df="${k}">${l} (${cnt(k)})</button>`).join("")}</div></div>
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

  function openUpgrade(pkg, from, to) {
    if (jobBusy()) return;
    openRun("", pkg, "upgrade");
    jobs.setMode("upgrade"); jobs.setUpgradeTo(to);
    const ov = pkgRepo[pkg];
    $("#run-title").innerHTML = `<span class="mono">upgrade</span> · ${esc(pkg)} <span class="draft-tag sj-c8ceca">USB'ye yazar</span>`;
    $("#run-sub").textContent = "KeyLab paket güncellemesi (imzalı indeks)";
    $("#run-confirm-text").innerHTML = `<b>${esc(pkg)}</b> ${from === "-" ? "kurulacak" : `<span class="mono">${esc(from)}</span> → <span class="mono">${esc(to)}</span> yükseltilecek`}: ${from === "-" ? `<span class="mono">${esc(to)}</span> sürümü ` : ""}indirilip <b>imzası ve özeti doğrulanarak</b> USB'deki <span class="mono">components/</span> klasörüne yazılacak. Eski sürüm klasörü silinmez; hiçbir sunucuya dokunulmaz.`;
    $("#run-confirm-extra").hidden = false;
    $("#run-confirm-extra").innerHTML = `<label class="small sj-c842b0"><input type="checkbox" id="u-ack"> <span>İndeksin gösterdiği kaynaktan${ov ? ` (<span class="mono">${esc(ov)}</span>)` : ""} indirilen sürümü kurmak istiyorum.</span></label>`;
    $("#run-confirm-note").textContent = "Çalıştır'a bastığınızda indeks yeniden alınıp imzası doğrulanır; gösterilen sürüm değişmişse kurulum durur. İmza, özet, depo eşleşmesi ve sürüm kapıları kurulum katmanının işidir; biri tutmazsa kurulum durur ve eski sürüm olduğu gibi kalır. Kurulum atomiktir, başladıktan sonra iptal edilemez.";
    $("#run-go").textContent = "Çalıştır"; $("#run-go").disabled = true;
    $("#u-ack").addEventListener("change", e => { $("#run-go").disabled = !e.target.checked; });
  }

  return { load: loadPackages, checkUpdates: checkUpdatesUI };
}
