/* KeyLab — Raporlar: şablonlu, yazdırılabilir güvenlik/envanter raporu (görsel, Markdown, JSON).
   pocketui'deki js/modules/reports.js'ten uyarlandı. Farklar: paket listesi sabit değil, yalnız mevcut uç noktalar (/api/home, /api/lynis,
   fark bölümü için js/compare.js verisi) kullanılır, tek bir veri modelinden üç çıktı üretilir, tüm hücreler kaçışlıdır, gizli görünen
   değişken değerleri maskelidir ("sıfır sızıntı" iddiası yok: yalnızca ad desenine göre maskeleme), yazdırma yalnız rapor belgesini basar. */
(function () {
  "use strict";
  const $ = (s, r = document) => r.querySelector(s);
  const $$ = (s, r = document) => Array.from(r.querySelectorAll(s));
  const esc = s => String(s ?? "").replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  const isEn = () => window.KeyLabI18n && window.KeyLabI18n.getLang() === "en";
  const L = (tr, en) => (isEn() ? en : tr);
  const CONC = 6;

  let hooks = null, fmt = "html", gen = 0, md = "", js = "";

  async function pool(items, fn) {
    let i = 0;
    await Promise.all(Array.from({ length: Math.min(CONC, items.length) }, async () => { while (i < items.length) { const it = items[i++]; await fn(it); } }));
  }

  const lastOf = (s, pid) => (s.recent || []).filter(a => a.package === pid).sort((a, b) => (a.time < b.time ? 1 : -1))[0] || (s.last && s.last.package === pid ? s.last : null);
  const fmtTime = iso => { const t = Date.parse(iso); return t ? new Date(t).toLocaleString(isEn() ? "en-GB" : "tr-TR", { dateStyle: "short", timeStyle: "short" }) : "—"; };

  const TEMPLATES = {
    "fleet-sec": () => [L("KeyLab — Filo güvenlik ve envanter raporu", "KeyLab — Fleet security and inventory report"), L("Sunucuların erişim, paket ve Lynis durumları", "Access, package and Lynis status of the servers")],
    "srv-audit": t => [L("KeyLab — Sunucu güvenlik denetimi: ", "KeyLab — Server security audit: ") + t, L("Seçili sunucu için güvenlik, paket ve bulgu özeti", "Security, package and findings summary for the selected server")],
    "lynis-catalog": () => [L("KeyLab — Lynis bulguları kataloğu", "KeyLab — Lynis findings catalog"), L("Denetimlerde saptanan uyarı ve öneriler", "Warnings and suggestions found by the audits")],
    "drift-report": () => [L("KeyLab — Yapılandırma ve drift raporu", "KeyLab — Configuration and drift report"), L("Sunucular arasındaki yapılandırma farkları", "Configuration differences between servers")],
  };

  function fillServers() {
    const sel = $("#rep-server"), home = hooks.getHome(), prev = sel.value;
    sel.innerHTML = "";
    const all = document.createElement("option"); all.value = "all"; all.textContent = L("Tüm filo (tüm sunucular)", "Entire fleet (all servers)"); sel.appendChild(all);
    for (const s of home.servers) { const o = document.createElement("option"); o.value = s.name; o.textContent = s.name; sel.appendChild(o); }
    sel.value = prev && (prev === "all" || home.servers.some(s => s.name === prev)) ? prev : "all";
  }

  // tek veri modeli: HTML, Markdown ve JSON bundan üretilir
  async function collect(my) {
    const home = hooks.getHome(), tpl = $("#rep-template").value, target = $("#rep-server").value;
    const inc = { sys: $("#rep-inc-sys").checked, pkgs: $("#rep-inc-pkgs").checked, lynis: $("#rep-inc-lynis").checked, findings: $("#rep-inc-findings").checked, vars: $("#rep-inc-vars").checked };
    const servers = target === "all" ? home.servers : home.servers.filter(s => s.name === target);
    const [title, subtitle] = TEMPLATES[tpl](target === "all" ? L("tüm filo", "entire fleet") : target);
    const data = { title, subtitle, generatedAt: new Date().toISOString(), target, template: tpl, servers: [], drift: null };
    for (const s of servers) {
      data.servers.push({
        name: s.name, host: s.host || "", port: s.port || "22", user: s.user || "", os: s.platform || s.os || "linux", state: hooks.stateText(s), up: !!(s.checked && s.up), knock: !!s.knock, hasKey: !!s.hasKey,
        lynis: s.lynis ? { score: s.lynis.score, warnings: s.lynis.warnings, suggestions: s.lynis.suggestions, date: s.lynis.date || "" } : null,
        packages: home.packages.map(p => { const a = lastOf(s, p.id); return { id: p.id, label: hooks.tabLabel(p.id), action: a ? a.action : "", ok: a ? a.result === "ok" : null, time: a ? a.time : "" }; }),
        findings: [],
      });
    }
    if (inc.findings && (tpl === "srv-audit" || tpl === "lynis-catalog" || target !== "all")) {
      await pool(data.servers, async s => {
        try { s.findings = ((await hooks.json("/api/lynis?server=" + encodeURIComponent(s.name))).findings || []).map(f => ({ id: f.id, kind: f.kind, text: f.text, suggestion: f.suggestion || "", value: f.value || "" })); } catch (e) { /* bulgu yok sayılır */ }
      });
    }
    if (my !== gen) return null;
    if (inc.vars && tpl === "drift-report") {
      const rows = await window.KeyLabCompare.getRows();
      if (my !== gen) return null;
      const ns = data.servers.map(s => s.name);
      data.drift = rows.filter(r => !r.section && r.drift).map(r => ({ label: r.label, values: Object.fromEntries(ns.map(n => [n, r.vals[n] ? r.vals[n].text : ""])) }));
    }
    return { data, inc, tpl };
  }

  /* ---------------- Görsel rapor ---------------- */
  function buildHtml(data, inc, tpl) {
    const S = data.servers, on = S.filter(s => s.up).length;
    const scores = S.filter(s => s.lynis && s.lynis.score > 0).map(s => s.lynis.score);
    const avg = scores.length ? Math.round(scores.reduce((a, b) => a + b, 0) / scores.length) : null;
    const warn = S.reduce((n, s) => n + (s.lynis ? s.lynis.warnings : 0), 0), sug = S.reduce((n, s) => n + (s.lynis ? s.lynis.suggestions : 0), 0);
    const kpi = (lbl, val, cls) => `<div class="rep-kpi-card"><div class="rep-kpi-lbl">${esc(lbl)}</div><div class="rep-kpi-val ${cls || ""}">${esc(val)}</div></div>`;
    const sec = (n, title, body) => `<div class="rep-section"><h2 class="rep-section-title">${n}. ${esc(title)}</h2>${body}</div>`;
    const tbl = (head, rows, empty) => `<table class="rep-table"><thead><tr>${head.map(h => `<th>${esc(h)}</th>`).join("")}</tr></thead><tbody>${rows.join("") || `<tr><td colspan="${head.length}" class="muted">${esc(empty)}</td></tr>`}</tbody></table>`;
    let n = 0, h = `<div class="rep-header"><div><h1 class="rep-title">${esc(data.title)}</h1><div class="rep-sub">${esc(data.subtitle)}</div></div>
      <div class="rep-meta-box"><div><b>${esc(L("Oluşturuldu", "Generated"))}:</b> ${esc(fmtTime(data.generatedAt))}</div><div><b>${esc(L("Kapsam", "Scope"))}:</b> ${esc(data.target === "all" ? L("Tüm filo", "Entire fleet") : data.target)}</div></div></div>
      <div class="rep-kpis">${kpi(L("Sunucu", "Servers"), S.length)}${kpi(L("Çevrimiçi", "Online"), `${on} / ${S.length}`, on === S.length ? "ok" : "warn")}${kpi(L("Ortalama Lynis", "Average Lynis"), avg === null ? "—" : avg, avg !== null && avg >= 70 ? "ok" : "warn")}${kpi(L("Toplam uyarı", "Total warnings"), warn, warn > 0 ? "warn" : "ok")}${kpi(L("Toplam öneri", "Total suggestions"), sug)}</div>`;
    if (inc.sys) h += sec(++n, L("Sunucu envanteri ve ağ", "Server inventory and network"), tbl([L("Sunucu", "Server"), L("Ana makine / IP", "Host / IP"), "Port", L("Kullanıcı", "User"), L("OS / profil", "OS / profile"), L("Durum", "Status"), "Knock"],
      S.map(s => `<tr><td><b>${esc(s.name)}</b></td><td class="mono">${esc(s.host || "—")}</td><td class="mono">${esc(s.port)}</td><td class="mono">${esc(s.user || "—")}</td><td>${esc(s.os)}</td><td>${esc(s.state)}</td><td>${s.knock ? "🔒" : "—"}</td></tr>`), "—"));
    if (inc.pkgs) h += sec(++n, L("Paket eylemleri (son durum)", "Package actions (latest)"), tbl([L("Sunucu", "Server"), L("Paket", "Package"), L("Son eylem", "Last action"), L("Sonuç", "Result"), L("Zaman", "Time")],
      S.flatMap(s => s.packages.map(p => `<tr><td><b>${esc(s.name)}</b></td><td>${esc(p.label)}</td><td>${esc(p.action || "—")}</td><td>${p.ok === null ? '<span class="muted">—</span>' : p.ok ? `<span class="res ok">✔ ${esc(L("başarılı", "ok"))}</span>` : `<span class="res fail">✖ ${esc(L("başarısız", "failed"))}</span>`}</td><td class="mono small">${esc(p.time ? fmtTime(p.time) : "—")}</td></tr>`)), L("Kayıtlı paket eylemi yok.", "No recorded package actions.")));
    if (inc.lynis) h += sec(++n, L("Lynis denetim özeti", "Lynis audit summary"), tbl([L("Sunucu", "Server"), L("Skor", "Score"), L("Uyarı", "Warnings"), L("Öneri", "Suggestions"), L("Son ölçüm", "Last audit")],
      S.map(s => s.lynis ? `<tr><td><b>${esc(s.name)}</b></td><td><b class="${s.lynis.score >= 70 ? "ok" : "warn"}">${esc(s.lynis.score)}</b> / 100</td><td>${esc(s.lynis.warnings)}</td><td>${esc(s.lynis.suggestions)}</td><td class="mono small">${esc(s.lynis.date ? s.lynis.date.slice(0, 16).replace("T", " ") : "—")}</td></tr>` : `<tr><td><b>${esc(s.name)}</b></td><td colspan="4" class="muted">${esc(L("Ölçüm yok", "No audit"))}</td></tr>`), "—"));
    const F = S.flatMap(s => s.findings.map(f => ({ ...f, server: s.name })));
    if (inc.findings && F.length) h += sec(++n, `${L("Bulgular", "Findings")} (${F.length})`, tbl([L("Sunucu", "Server"), L("Tür", "Type"), "ID", L("Açıklama / öneri", "Description / suggestion")],
      F.map(f => `<tr><td><b>${esc(f.server)}</b></td><td>${esc(f.kind === "uyari" ? L("UYARI", "WARNING") : L("ÖNERİ", "SUGGESTION"))}</td><td class="mono">${esc(f.id)}</td><td><div>${esc(f.text)}</div>${f.suggestion ? `<div class="small muted">${esc(L("Öneri", "Suggestion"))}: ${esc(f.suggestion)}</div>` : ""}${f.value ? `<div class="small mono muted">${esc(f.value)}</div>` : ""}</td></tr>`), "—"));
    if (data.drift) h += sec(++n, L("Yapılandırma farkları (drift)", "Configuration differences (drift)"), `<div class="small muted" style="margin-bottom:8px">${esc(L("Yalnızca sunucular arasında değeri farklı olan satırlar. Gizli görünen değişkenler maskelidir.", "Only rows whose value differs between servers. Variables that look secret are masked."))}</div>` + tbl([L("Parametre", "Parameter"), ...S.map(s => s.name)],
      data.drift.map(r => `<tr><td><b>${esc(r.label)}</b></td>${S.map(s => `<td class="mono">${esc(r.values[s.name] || "—")}</td>`).join("")}</tr>`), L("Fark algılanmadı.", "No differences detected.")));
    return h + `<div class="small muted rep-foot">${esc(L("Bu rapor KeyLab tarafından bu makinedeki verilerle yerelde üretildi. Adı gizli bilgi çağrıştıran değişkenlerin değerleri maskelenir.", "This report was generated locally by KeyLab from data on this machine. Values of variables whose names suggest secrets are masked."))}</div>`;
  }

  /* ---------------- Markdown ---------------- */
  const mdt = s => String(s ?? "").replace(/[\r\n]+/g, " ").replace(/[<>]/g, c => "\\" + c); // ham HTML etiketi Markdown'da çalışmasın
  const mdc = s => mdt(String(s ?? "").replace(/\\/g, "\\\\").replace(/\|/g, "\\|"));
  function buildMd(data, inc) {
    const S = data.servers; let o = `# ${data.title}\n*${data.subtitle}*\n\n- **${L("Oluşturuldu", "Generated")}:** ${data.generatedAt}\n- **${L("Kapsam", "Scope")}:** ${data.target === "all" ? L("Tüm filo", "Entire fleet") : data.target}\n- **${L("Sunucu", "Servers")}:** ${S.length} (${S.filter(s => s.up).length} ${L("çevrimiçi", "online")})\n\n---\n\n`;
    const table = (head, rows) => `| ${head.map(mdc).join(" | ")} |\n| ${head.map(() => "---").join(" | ")} |\n${rows.map(r => `| ${r.map(mdc).join(" | ")} |`).join("\n")}\n\n`;
    if (inc.sys) o += `## ${L("Sunucu envanteri ve ağ", "Server inventory and network")}\n\n` + table([L("Sunucu", "Server"), "Host/IP", "Port", L("Kullanıcı", "User"), "OS", L("Durum", "Status"), "Knock"], S.map(s => [s.name, s.host, s.port, s.user, s.os, s.state, s.knock ? "🔒" : "—"]));
    if (inc.pkgs) o += `## ${L("Paket eylemleri (son durum)", "Package actions (latest)")}\n\n` + table([L("Sunucu", "Server"), L("Paket", "Package"), L("Son eylem", "Last action"), L("Sonuç", "Result"), L("Zaman", "Time")], S.flatMap(s => s.packages.map(p => [s.name, p.label, p.action || "—", p.ok === null ? "—" : p.ok ? "ok" : "fail", p.time || "—"])));
    if (inc.lynis) o += `## ${L("Lynis denetim özeti", "Lynis audit summary")}\n\n` + table([L("Sunucu", "Server"), L("Skor", "Score"), L("Uyarı", "Warnings"), L("Öneri", "Suggestions"), L("Son ölçüm", "Last audit")], S.map(s => s.lynis ? [s.name, s.lynis.score + "/100", s.lynis.warnings, s.lynis.suggestions, s.lynis.date ? s.lynis.date.slice(0, 10) : "—"] : [s.name, "—", "—", "—", L("ölçüm yok", "no audit")]));
    const F = S.flatMap(s => s.findings.map(f => ({ ...f, server: s.name })));
    if (inc.findings && F.length) { o += `## ${L("Bulgular", "Findings")} (${F.length})\n\n`; for (const f of F) { o += `- **[${f.kind === "uyari" ? L("UYARI", "WARNING") : L("ÖNERİ", "SUGGESTION")}]** \`${String(f.id).replace(/`/g, "")}\` (*${f.server}*): ${mdt(f.text)}\n`; if (f.suggestion) o += `  - ${L("Öneri", "Suggestion")}: ${mdt(f.suggestion)}\n`; } o += "\n"; }
    if (data.drift) o += `## ${L("Yapılandırma farkları (drift)", "Configuration differences (drift)")}\n\n` + table([L("Parametre", "Parameter"), ...S.map(s => s.name)], data.drift.map(r => [r.label, ...S.map(s => r.values[s.name] || "—")]));
    return o + `---\n*${L("KeyLab tarafından yerelde üretildi.", "Generated locally by KeyLab.")}*\n`;
  }

  function showFmt() {
    $("#rep-render-html").hidden = fmt !== "html";
    const raw = $("#rep-render-raw"); raw.hidden = fmt === "html"; raw.textContent = fmt === "md" ? md : fmt === "json" ? js : "";
    $$("#rep-format-toggle button").forEach(b => b.setAttribute("aria-pressed", String(b.dataset.fmt === fmt)));
  }

  async function render() {
    const my = ++gen, box = $("#rep-render-html");
    box.innerHTML = `<div class="small muted" style="padding:24px;text-align:center">${esc(L("Rapor hazırlanıyor…", "Preparing the report…"))}</div>`;
    const r = await collect(my);
    if (!r || my !== gen) return;
    $("#rep-preview-title").textContent = r.data.title;
    box.innerHTML = buildHtml(r.data, r.inc, r.tpl);
    md = buildMd(r.data, r.inc); js = JSON.stringify(r.data, null, 2);
    showFmt();
  }

  function load() { fillServers(); return render(); }

  function print() {
    const prev = fmt; fmt = "html"; showFmt();
    const done = () => { document.body.classList.remove("print-report"); window.removeEventListener("afterprint", done); fmt = prev; showFmt(); };
    document.body.classList.add("print-report"); window.addEventListener("afterprint", done); window.print();
  }

  function download(name, text, type) {
    const a = document.createElement("a"), url = URL.createObjectURL(new Blob([text], { type }));
    a.href = url; a.download = name; document.body.appendChild(a); a.click(); a.remove(); setTimeout(() => URL.revokeObjectURL(url), 2000);
  }

  function init(h) {
    hooks = h;
    ["#rep-template", "#rep-server", "#rep-inc-sys", "#rep-inc-pkgs", "#rep-inc-lynis", "#rep-inc-findings", "#rep-inc-vars"].forEach(id => $(id).addEventListener("change", render));
    $$("#rep-format-toggle button").forEach(b => b.addEventListener("click", () => { fmt = b.dataset.fmt || "html"; showFmt(); }));
    $("#rep-print-btn").addEventListener("click", print);
    $("#rep-copy-md").addEventListener("click", async () => { try { await navigator.clipboard.writeText(md); hooks.toast(L("Markdown kopyalandı", "Markdown copied")); } catch (e) { hooks.toast(L("Kopyalanamadı", "Could not copy")); } });
    const day = () => new Date().toISOString().slice(0, 10);
    $("#rep-down-md").addEventListener("click", () => download(`keylab-rapor-${day()}.md`, md, "text/markdown;charset=utf-8"));
    $("#rep-down-json").addEventListener("click", () => download(`keylab-rapor-${day()}.json`, js, "application/json;charset=utf-8"));
    window.addEventListener("keylab:langchange", () => { if (!$("#view-reports").hidden) load(); });
  }

  window.KeyLabReports = { init, load };
})();
