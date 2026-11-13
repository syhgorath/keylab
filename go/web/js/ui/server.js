/* KeyLab — Sunucu sayfası: başlık düğmeleri, sekmeler, Özet (durum, Lynis, hazırlık), paket sekmesi (Lynis kahramanı, Son durum,
   değişken kartı, eylem kartları + süzgeç), yazılım kataloğu (`items`), adres düzenleme, port knocking bağlantısı, openServer.
   app.js'ten çıkarıldı (refaktör 5d), kod BİREBİR; dış bağlantılar hooks ile. Testler: go/webtest/server1-3.test.js + sayfa görüntüleri */
import { $, $$, esc, json, toast, ago, localTime, isOk } from "../lib/util.js";
import { appState } from "../state.js";
import { TAB_LABEL } from "../lib/labels.js";
import { OS_LABEL, osIcon } from "../lib/icons.js";
import { chart } from "./lynis.js";
import { initKnock } from "./knock.js";

// hooks: { openRun, openWrite, openTerm — iş penceresi; openVars, varCur — değişken penceresi; openDlg, closeDlg — ortak pencere;
//          openTask(sunucu, görev) — hazırlık görevi; openCtx(x, y, ad) — sunucu bağlam menüsü; showView, load, renderSrvTree, setHash, stateText;
//          logs, lynis — Günlükler/Lynis sayfası API'leri }
//   →  { openServer, renderServer, goLogs, goLynis, openEditAddr, act, resetCatalog }
export function initServer(hooks) {
  const { openRun, openWrite, openTerm, openVars, varCur, openDlg, closeDlg, openTask, openCtx, showView, load, renderSrvTree, setHash, stateText, logs, lynis } = hooks;

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

  return { openServer, renderServer, goLogs, goLynis, openEditAddr, act, resetCatalog: () => { catSrv = ""; } };
}
