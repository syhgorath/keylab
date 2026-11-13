/* KeyLab — Günlükler sayfası: süzgeç düğmeleri, dosya listesi, günlük görüntüleyici (arama, hata satırına git). app.js'ten çıkarıldı
   (refaktör 5), kod BİREBİR. Testler: go/webtest/lynlogs.test.js */
import { $, $$, esc, api, json, toast, ago, localTime } from "../lib/util.js";
import { appState } from "../state.js";
import { TAB_LABEL } from "../lib/labels.js";
import { osIcon } from "../lib/icons.js";

// hooks: { showView(ad) }  →  { load(), go(sunucu), openLast(paket, sunucu) }
export function initLogs(hooks) {
  const { showView } = hooks;
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
        <input class="sj-361285" id="logq" type="search" placeholder="dosya adında ara…" aria-label="Günlük dosyalarında ara" value="${esc(logQ)}">
        ${(logSrv || logPkg || logKind !== "all" || logQ) ? '<button class="btn sm ghost" id="logclear">Süzgeçleri temizle</button>' : ""}</div></div>`;
    $("#logs-count").textContent = `${filtered.length} / ${logAll.length} dosya`;
    $("#loglist").innerHTML = filtered.map((l, i) => `<button data-i="${i}" aria-current="${logId(l) === logKey}"><span class="mono">${osIconSmall(l.server)}${esc(l.server)} · ${esc(l.package)} · ${esc(l.file)}</span>
      <span class="muted small" title="${esc(localTime(l.mtime))}">${esc(ago(l.mtime))} · ${Math.max(1, Math.round(l.size / 1024))} KB</span></button>`).join("") || '<div class="small muted sj-cc781b">Süzgece uyan günlük yok.</div>';
    $$("#loglist button[data-i]").forEach(b => b.addEventListener("click", () => openLog(filtered[+b.dataset.i], b)));
    $$("#logfilters [data-srv]").forEach(b => b.addEventListener("click", () => { logSrv = b.dataset.srv; paintLogs(); }));
    $$("#logfilters [data-pkg]").forEach(b => b.addEventListener("click", () => { logPkg = b.dataset.pkg; paintLogs(); }));
    $$("#logfilters [data-kind]").forEach(b => b.addEventListener("click", () => { logKind = b.dataset.kind; paintLogs(); }));
    const q = $("#logq"); q.addEventListener("input", () => { logQ = q.value.trim().toLowerCase(); const pos = q.selectionStart; paintLogs(); const n = $("#logq"); n.focus(); n.setSelectionRange(pos, pos); });
    const c = $("#logclear"); if (c) c.addEventListener("click", () => { logSrv = logPkg = logQ = ""; logKind = "all"; paintLogs(); });
  }
  // sunucu adına göre küçük simge (liste satırı için)
  const osIconSmall = name => { const s = appState.home.servers.find(x => x.name === name); return s ? osIcon(s.os, 16) + " " : ""; };

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

  function go(server) { logSrv = server; logPkg = ""; logKind = "all"; logQ = ""; showView("logs"); }
  // paketin bu sunucudaki son çalıştırma günlüğünü Günlükler sayfasında aç
  async function openLast(pkg, server) {
    showView("logs"); logSrv = server; logPkg = pkg; logKind = "last"; logQ = "";
    await loadLogs();
    const l = logAll.find(x => x.package === pkg && x.server === server && x.file === "last-run.log");
    if (l) { const i = $$("#loglist button[data-i]").findIndex(b => b.textContent.includes("last-run.log")); await openLog(l, $$("#loglist button[data-i]")[i]); }
  }
  return { load: loadLogs, go, openLast };
}
