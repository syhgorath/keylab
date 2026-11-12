/* KeyLab — Sunucu karşılaştırma ve drift matrisi.
   pocketui'deki js/modules/compare.js'ten uyarlandı. Farklar: paket listesi sabit değil (home.packages), yalnız mevcut uç noktalar
   (/api/home verisi + /api/vars) kullanılır, her hücre kaçışlıdır, gizli görünen değişken adları (parola/token/anahtar) ekranda ve
   dışa aktarımda maskelenir (karşılaştırma yine gerçek değere göre yapılır), CSV hücreleri formül enjeksiyonuna karşı korunur. */
(function () {
  "use strict";
  const $ = (s, r = document) => r.querySelector(s);
  const $$ = (s, r = document) => Array.from(r.querySelectorAll(s));
  const esc = s => String(s ?? "").replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  const isEn = () => window.KeyLabI18n && window.KeyLabI18n.getLang() === "en";
  const L = (tr, en) => (isEn() ? en : tr);

  const SECRET = /(pass|secret|token|private|credential)/i;
  const CONC = 6, TTL = 60000;
  let hooks = null, driftOnly = false, cat = "all", q = "", rows = [], details = {}, fetchedAt = 0, gen = 0, pkgIds = [];

  const show = (name, v) => (SECRET.test(name) && v !== "" ? "••••" : v);

  // en çok CONC eşzamanlı istek
  async function pool(items, fn) {
    let i = 0;
    await Promise.all(Array.from({ length: Math.min(CONC, items.length) }, async () => { while (i < items.length) { const it = items[i++]; await fn(it); } }));
  }

  async function load(force) {
    const el = $("#compare-matrix-view"); if (!el) return;
    const home = hooks.getHome();
    if (!home.servers.length) { el.innerHTML = `<div class="small muted" style="padding:32px;text-align:center">${esc(L("Henüz kayıtlı sunucu yok.", "No servers registered yet."))}</div>`; $("#compare-summary").textContent = ""; return; }
    if (!force && Date.now() - fetchedAt < TTL && home.servers.every(s => details[s.name])) { render(); return; }
    const my = ++gen;
    el.innerHTML = `<div class="small muted" style="padding:28px;text-align:center">${esc(L("Değişkenler okunuyor…", "Reading variables…"))}</div>`;
    pkgIds = home.packages.map(p => p.id);
    const jobs = [];
    for (const s of home.servers) for (const p of pkgIds) jobs.push([s.name, p]);
    const next = {};
    home.servers.forEach(s => { next[s.name] = {}; });
    await pool(jobs, async ([sn, pid]) => {
      try { next[sn][pid] = (await hooks.json(`/api/vars?server=${encodeURIComponent(sn)}&package=${encodeURIComponent(pid)}`)).vars || []; }
      catch (e) { next[sn][pid] = []; }
    });
    if (my !== gen) return; // daha yeni bir yükleme başladı
    details = next; fetchedAt = Date.now();
    render();
  }

  function makeRow(c, label, extract) {
    const servers = hooks.getHome().servers, vals = {}, counts = {};
    for (const s of servers) { const v = extract(s); vals[s.name] = v; counts[v.raw] = (counts[v.raw] || 0) + 1; }
    const distinct = Object.keys(counts), drift = distinct.length > 1;
    let major = distinct[0], max = -1;
    for (const [k, n] of Object.entries(counts)) if (n > max) { max = n; major = k; }
    for (const s of servers) vals[s.name].diff = drift && vals[s.name].raw !== major;
    return { cat: c, label, drift, vals };
  }

  const lastOf = (s, pid) => (s.recent || []).filter(a => a.package === pid).sort((a, b) => (a.time < b.time ? 1 : -1))[0] || (s.last && s.last.package === pid ? s.last : null);

  function build() {
    const home = hooks.getHome(), servers = home.servers, out = [];
    const sec = (c, label) => out.push({ section: true, cat: c, label });
    const cell = (raw, html, text) => ({ raw: String(raw), html, text: text === undefined ? String(raw) : text });

    sec("sys", L("1. Sistem ve ağ", "1. System and network"));
    out.push(makeRow("sys", L("Ağ durumu", "Network status"), s => {
      const st = hooks.dotState(s), tx = hooks.stateText(s);
      return cell(st, `<span class="status-dot ${esc(st)}" aria-hidden="true"></span> ${esc(tx)}`, tx);
    }));
    out.push(makeRow("sys", L("Ana makine / IP", "Hostname / IP"), s => cell(s.host || "", `<span class="mono">${esc(s.host || "—")}</span>`)));
    out.push(makeRow("sys", L("SSH portu", "SSH port"), s => cell(s.port || "22", `<span class="mono">${esc(s.port || "22")}</span>`)));
    out.push(makeRow("sys", L("SSH kullanıcısı", "SSH user"), s => cell(s.user || "", `<span class="mono">${esc(s.user || "—")}</span>`)));
    out.push(makeRow("sys", L("OS / profil", "OS / profile"), s => cell(s.os || "", `<span class="row-flex" style="gap:5px">${hooks.osIcon(s.os, 16)} <span>${esc(s.platform || s.os || "linux")}</span></span>`, s.platform || s.os || "")));
    out.push(makeRow("sys", L("SSH anahtarı", "SSH key"), s => cell(s.hasKey ? "var" : "yok", s.hasKey ? `<span class="res ok">✔ ${esc(L("var", "present"))}</span>` : `<span class="muted">— ${esc(L("yok", "none"))}</span>`, s.hasKey ? L("var", "present") : L("yok", "none"))));
    out.push(makeRow("sys", "Port knocking", s => cell(s.knock ? "açık" : "kapalı", s.knock ? `<span class="badge warn">🔒 ${esc(L("etkin", "enabled"))}</span>` : `<span class="muted">— ${esc(L("kapalı", "off"))}</span>`, s.knock ? L("etkin", "enabled") : L("kapalı", "off"))));

    sec("pkg", L("2. Paket eylemleri (son durum)", "2. Package actions (latest)"));
    for (const pid of pkgIds) {
      out.push(makeRow("pkg", `${hooks.tabLabel(pid)} ${L("(son eylem)", "(last action)")}`, s => {
        const a = lastOf(s, pid);
        if (!a) return cell("—", `<span class="muted">— ${esc(L("yapılmadı", "never run"))}</span>`, L("yapılmadı", "never run"));
        const ok = a.result === "ok";
        return cell(`${a.action}:${ok ? "ok" : "fail"}`, `<span class="res ${ok ? "ok" : "fail"}">${ok ? "✔" : "✖"} ${esc(a.action)}</span> <span class="muted mono small">${esc(hooks.ago(a.time))}</span>`, `${a.action} ${ok ? "ok" : "fail"}`);
      }));
    }

    sec("lynis", L("3. Lynis denetimi", "3. Lynis audit"));
    out.push(makeRow("lynis", L("Güvenlik skoru", "Security score"), s => {
      const l = s.lynis; if (!l) return cell("0", `<span class="muted">— ${esc(L("ölçüm yok", "no audit"))}</span>`, "—");
      const c = l.score >= 75 ? "ok" : l.score >= 55 ? "warn" : "fail";
      return cell(l.score, `<b class="${c}">${esc(l.score)}</b><span class="muted small">/100</span>`);
    }));
    for (const [key, kind, trL, enL, cls] of [["warnings", "uyari", "Uyarı sayısı", "Warnings", "warn"], ["suggestions", "oneri", "Öneri sayısı", "Suggestions", "info"]]) {
      out.push(makeRow("lynis", L(trL, enL), s => {
        const l = s.lynis; if (!l) return cell("0", '<span class="muted">—</span>', "—");
        const n = l[key];
        return cell(n, n > 0 ? `<button type="button" class="badge ${cls}" data-lynis="${esc(kind)}" data-server="${esc(s.name)}">${esc(n)} ↗</button>` : `<span class="res ok">0</span>`);
      }));
    }
    out.push(makeRow("lynis", L("Son denetim tarihi", "Last audit date"), s => {
      const d = s.lynis && s.lynis.date ? s.lynis.date.slice(0, 10) : "";
      return d ? cell(d, `<span class="mono small muted">${esc(d)}</span>`) : cell("—", '<span class="muted">—</span>');
    }));

    for (const pid of pkgIds) {
      const names = new Set(), sample = {};
      for (const s of servers) for (const v of (details[s.name] && details[s.name][pid]) || []) { names.add(v.name); if (!sample[v.name]) sample[v.name] = v; }
      if (!names.size) continue;
      sec("vars", `${L("4. Değişkenler", "4. Variables")} · ${hooks.tabLabel(pid)}`);
      for (const vn of [...names].sort()) {
        const r = makeRow("vars", vn, s => {
          const v = ((details[s.name] && details[s.name][pid]) || []).find(x => x.name === vn);
          if (!v) return { raw: "(tanımsız)", html: `<span class="muted small">(${esc(L("yok", "none"))})</span>`, text: "(tanımsız)", unset: true };
          const cur = String(v.set ? v.value : v.default);
          return { raw: cur, html: `<span class="matrix-cell-val matrix-cell-clickable" role="button" tabindex="0" data-var="${esc(s.name)}" data-pkg="${esc(pid)}" data-group="${esc(sample[vn].group || "")}" title="${esc(L("Düzenle", "Edit"))}">${esc(show(vn, cur))}</span>`, text: show(vn, cur) };
        });
        r.desc = sample[vn].description || ""; out.push(r);
      }
    }
    return out;
  }

  function render() {
    const el = $("#compare-matrix-view"); if (!el) return;
    const home = hooks.getHome(), servers = home.servers; if (!servers.length) return;
    rows = build();
    const needle = q.toLowerCase(), filtered = [];
    let curSec = null, bucket = [];
    const flush = () => { if (curSec && bucket.length) { filtered.push(curSec, ...bucket); } curSec = null; bucket = []; };
    for (const r of rows) {
      if (r.section) { flush(); curSec = r; continue; }
      if ((cat === "all" || r.cat === cat) && (!driftOnly || r.drift) && (!needle || r.label.toLowerCase().includes(needle) || Object.values(r.vals).some(v => v.text.toLowerCase().includes(needle)))) bucket.push(r);
    }
    flush();
    const nDrift = rows.filter(r => !r.section && r.drift).length;
    $("#compare-summary").innerHTML = isEn() ? `<b>${servers.length}</b> servers · <b>${nDrift}</b> parameters differ` : `<b>${servers.length}</b> sunucu · <b>${nDrift}</b> parametrede fark var`;
    let h = `<div class="matrix-wrap"><table class="matrix-table" aria-label="${esc(L("Sunucu karşılaştırma matrisi", "Server comparison matrix"))}"><thead><tr><th class="sticky-col">${esc(L("Parametre", "Parameter"))}</th>${servers.map(s => `<th><div class="srv-col-head"><span class="status-dot ${esc(hooks.dotState(s))}" aria-hidden="true"></span>${hooks.osIcon(s.os, 16)}<a href="#" class="plain-link" data-open="${esc(s.name)}">${esc(s.name)}</a></div></th>`).join("")}</tr></thead><tbody>`;
    if (!filtered.length) h += `<tr><td colspan="${servers.length + 1}" class="muted" style="text-align:center;padding:32px">${esc(driftOnly ? L("Seçili süzgeçte fark yok: yapılandırmalar aynı.", "No differences in this filter: configurations match.") : L("Aramayla eşleşen parametre yok.", "No parameter matches the search."))}</td></tr>`;
    for (const r of filtered) {
      if (r.section) { h += `<tr class="matrix-section-row"><td colspan="${servers.length + 1}" class="sticky-col">${esc(r.label)}</td></tr>`; continue; }
      h += `<tr class="matrix-row${r.drift ? " has-drift" : ""}"><td class="sticky-col" title="${esc(r.desc || r.label)}"><span>${esc(r.label)}</span>${r.drift ? `<span class="drift-badge" title="${esc(L("Sunucular arasında değerler uyuşmuyor", "Values differ between servers"))}">⚡ DRIFT</span>` : ""}</td>${servers.map(s => { const v = r.vals[s.name]; return `<td${r.drift && v.diff ? ' class="diff-val"' : ""}>${v.html}</td>`; }).join("")}</tr>`;
    }
    el.innerHTML = h + "</tbody></table></div>";
    $$("[data-open]", el).forEach(a => a.addEventListener("click", e => { e.preventDefault(); hooks.openServer(a.dataset.open, "ozet"); }));
    $$("[data-lynis]", el).forEach(b => b.addEventListener("click", () => hooks.goLynis(b.dataset.server, b.dataset.lynis)));
    $$("[data-var]", el).forEach(b => {
      const go = () => hooks.openVars(b.dataset.var, b.dataset.pkg, b.dataset.group || undefined);
      b.addEventListener("click", go);
      b.addEventListener("keydown", e => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); go(); } });
    });
  }

  /* ---------------- Dışa aktarma ---------------- */
  const names = () => hooks.getHome().servers.map(s => s.name);
  const csvCell = s => { let v = String(s ?? ""); if (/^[=+\-@\t\r]/.test(v)) v = "'" + v; return '"' + v.replace(/"/g, '""') + '"'; }; // formül enjeksiyonu: Excel/Sheets
  const mdCell = s => String(s ?? "").replace(/\\/g, "\\\\").replace(/\|/g, "\\|").replace(/[\r\n]+/g, " ");
  const cellText = (r, n) => (r.vals[n] ? r.vals[n].text : "");

  function toMarkdown() {
    const ns = names();
    let md = `# KeyLab — ${L("Sunucu karşılaştırma ve drift matrisi", "Server comparison and drift matrix")}\n\n*${new Date().toISOString()}* · ${ns.length} ${L("sunucu", "servers")}\n\n`;
    md += `| ${L("Parametre", "Parameter")} | Drift | ${ns.map(mdCell).join(" | ")} |\n| --- | :---: | ${ns.map(() => "---").join(" | ")} |\n`;
    for (const r of rows) md += r.section ? `| **${mdCell(r.label)}** | | ${ns.map(() => "").join(" | ")} |\n` : `| ${mdCell(r.label)} | ${r.drift ? "⚡" : "—"} | ${ns.map(n => mdCell(cellText(r, n))).join(" | ")} |\n`;
    return md;
  }
  function toCSV() {
    const ns = names();
    let csv = [L("Parametre", "Parameter"), "Drift", ...ns].map(csvCell).join(",") + "\r\n";
    for (const r of rows) csv += (r.section ? ["--- " + r.label + " ---", "", ...ns.map(() => "")] : [r.label, r.drift ? "DRIFT" : "OK", ...ns.map(n => cellText(r, n))]).map(csvCell).join(",") + "\r\n";
    return csv;
  }
  function toJSON() {
    const ns = names();
    return JSON.stringify({ generatedAt: new Date().toISOString(), servers: hooks.getHome().servers.map(s => ({ name: s.name, host: s.host, port: s.port, user: s.user, os: s.os })),
      rows: rows.map(r => r.section ? { section: r.label } : { parameter: r.label, category: r.cat, drift: r.drift, values: Object.fromEntries(ns.map(n => [n, cellText(r, n)])) }) }, null, 2);
  }
  function download(name, text, type) {
    const a = document.createElement("a"), url = URL.createObjectURL(new Blob([text], { type }));
    a.href = url; a.download = name; document.body.appendChild(a); a.click(); a.remove(); setTimeout(() => URL.revokeObjectURL(url), 2000);
  }

  function init(h) {
    hooks = h;
    $("#compare-drift-only").addEventListener("change", e => { driftOnly = e.target.checked; render(); });
    $("#compare-search").addEventListener("input", e => { q = e.target.value.trim(); render(); });
    $$("#compare-cat-group button").forEach(b => b.addEventListener("click", () => {
      $$("#compare-cat-group button").forEach(x => x.setAttribute("aria-pressed", String(x === b)));
      cat = b.dataset.cat || "all"; render();
    }));
    $("#compare-refresh-btn").addEventListener("click", () => { load(true); hooks.toast(L("Yenileniyor…", "Refreshing…")); });
    const menu = $("#compare-export-menu");
    $("#compare-export-btn").addEventListener("click", e => { e.stopPropagation(); menu.hidden = !menu.hidden; });
    document.addEventListener("click", e => { if (!menu.hidden && !e.target.closest(".dropdown-wrap")) menu.hidden = true; });
    document.addEventListener("keydown", e => { if (e.key === "Escape") menu.hidden = true; });
    const day = () => new Date().toISOString().slice(0, 10);
    $("#compare-copy-md").addEventListener("click", async () => {
      menu.hidden = true;
      try { await navigator.clipboard.writeText(toMarkdown()); hooks.toast(L("Markdown kopyalandı", "Markdown copied")); } catch (e) { hooks.toast(L("Kopyalanamadı", "Could not copy")); }
    });
    $("#compare-down-csv").addEventListener("click", () => { menu.hidden = true; download(`keylab-drift-${day()}.csv`, "\uFEFF" + toCSV(), "text/csv;charset=utf-8"); });
    $("#compare-down-json").addEventListener("click", () => { menu.hidden = true; download(`keylab-drift-${day()}.json`, toJSON(), "application/json;charset=utf-8"); });
    window.addEventListener("keylab:langchange", () => { if (rows.length) render(); });
  }

  // sunucu/paket listesi değişince (ekle, sil, yeniden adlandır) önbellek geçersiz
  // rapor modülü için: fark satırları (gerekirse değişkenler yüklenir)
  async function getRows() { await load(false); return rows; }
  window.KeyLabCompare = { init, load, getRows, invalidate: () => { fetchedAt = 0; details = {}; } };
})();
