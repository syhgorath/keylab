/* KeyLab — Lynis sayfası: sunucu seçici, skor geçmişi grafiği, bulgu süzgeci/araması. app.js'ten çıkarıldı (refaktör 5), kod BİREBİR;
   dışa verilen chart(l) Pano kartı ve sunucu özeti tarafından da kullanılır. Testler: go/webtest/lynlogs.test.js */
import { $, $$, esc, json } from "../lib/util.js";
import { appState } from "../state.js";
import { osIcon } from "../lib/icons.js";

export function chart(l) {
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

// hooks: { showView(ad) }  →  { load(ad?), go(sunucu, tür), repaint() }
export function initLynis(hooks) {
  const { showView } = hooks;
  let lynSel = "", lynData = null, lynKind = "all", lynQ = "", lynPre = ""; // lynPre: sayfa açılırken seçilecek tür süzgeci

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
        <div class="card-body"><div class="lynis-head"><div class="lynis-score">${l.score}</div><div class="small muted sj-9da19d">/100</div>
          ${l.history.length > 1 ? `<div class="lynis-delta">${delta >= 0 ? "▲ +" : "▼ "}${delta}</div>` : ""}</div>
          <div class="lynis-meta"><span class="badge warn">uyarı ${l.warnings}</span><span class="badge info">öneri ${l.suggestions}</span>${l.history.length > 1 ? `<span class="badge plain">${first} → ${l.score}</span>` : ""}</div>
          <div class="chartbox">${chart(l)}</div>
          <div class="small muted sj-fe7b49">Geçmiş, bu kontrolcüde kayıtlı tüm ölçümleri içerir; VM snapshot dönüşlerinden önceki ölçümler de dahildir.</div></div></div>
      <div class="card"><div class="card-head"><h2>Bulgular</h2><div class="spacer"></div>
        <div class="segmented" role="group" aria-label="Tür"><button data-k="all" aria-pressed="${lynKind === "all"}">Tümü</button><button data-k="uyari" aria-pressed="${lynKind === "uyari"}">Uyarı</button><button data-k="oneri" aria-pressed="${lynKind === "oneri"}">Öneri</button></div>
        <input class="sj-ab2990" id="lyn-q" type="search" value="${esc(lynQ)}" placeholder="ara…" aria-label="Bulgularda ara"><span class="hint sj-5dd2a6" id="lyn-count"></span></div>
        <div class="card-body tight"><div class="fgrid" id="lyn-rows"></div></div></div>`;
    $$("#lynis-detail .segmented button").forEach(b => b.addEventListener("click", () => { lynKind = b.dataset.k; $$("#lynis-detail .segmented button").forEach(x => x.setAttribute("aria-pressed", String(x === b))); paintFindings(); }));
    $("#lyn-q").addEventListener("input", e => { lynQ = e.target.value.trim(); paintFindings(); });
    paintFindings();
  }

  async function loadLynis(name) {
    if (!appState.home.servers.length) { $("#lynis-pick").innerHTML = ""; $("#lynis-detail").innerHTML = '<div class="card"><div class="card-body"><div class="empty-state"><h3>Sunucu yok</h3><p>Lynis ölçümü için önce bir sunucu eklenmeli.</p></div></div></div>'; return; }
    const have = appState.home.servers.filter(s => s.lynis).sort((a, b) => (b.lynis.date || "").localeCompare(a.lynis.date || ""));
    if (name) lynSel = name;
    if (!lynSel || !appState.home.servers.some(s => s.name === lynSel)) lynSel = (have[0] || appState.home.servers[0] || {}).name || appState.selected || "";
    $("#lynis-pick").innerHTML = appState.home.servers.map(s => `<button class="pickbtn" role="tab" aria-selected="${s.name === lynSel}" data-n="${esc(s.name)}">${osIcon(s.os, 26)}<span>${esc(s.name)}${s.lynis ? ` <b>${s.lynis.score}</b>` : ' <span class="muted">ölçüm yok</span>'}</span></button>`).join("");
    $$("#lynis-pick .pickbtn").forEach(b => b.addEventListener("click", () => loadLynis(b.dataset.n)));
    if (!lynSel) { $("#lynis-detail").innerHTML = ""; return; }
    lynKind = lynPre || "all"; lynPre = ""; lynQ = "";
    try { lynData = await json("/api/lynis?server=" + encodeURIComponent(lynSel)); lynData.findings = lynData.findings || []; } // sunucu bulgu dosyası yoksa null gönderir
    catch (e) { $("#lynis-detail").innerHTML = `<div class="notice danger"><span class="ico" aria-hidden="true">✕</span><div>Yüklenemedi: ${esc(e.message)}</div></div>`; return; }
    paintLynis();
  }

  function go(server, kind) { lynSel = server; lynPre = kind || ""; showView("lynis"); }
  const repaint = () => { if (lynData) paintLynis(); return !!lynData; };   // yeniden boyutlandırmada grafik genişliği değişir
  return { load: loadLynis, go, repaint };
}
