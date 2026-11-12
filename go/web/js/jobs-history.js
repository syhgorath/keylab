/* KeyLab — Son işler: sunucunun bellekte tuttuğu son işlerin (çalışan dahil) listesi; satıra tıklayınca çıktısı açılır.
   Veri kaynağı GET /api/jobs (çıktısız özet) ve GET /api/run?id= (çıktı, sunucuda maskelenmiş). Salt okunur: iş başlatmaz/durdurmaz.
   Bellekte en çok 20 iş kalır ve sunucu kapanınca silinir; kalıcı geçmiş Günlükler sayfasındadır. */
(function () {
  "use strict";
  const $ = (s, r = document) => r.querySelector(s);
  const $$ = (s, r = document) => Array.from(r.querySelectorAll(s));
  const { html, setHtml } = window.KeyLabHtml;
  const isEn = () => window.KeyLabI18n && window.KeyLabI18n.getLang() === "en";
  const L = (tr, en) => (isEn() ? en : tr);
  let hooks = null, timer = 0, jobs = [], running = "", openId = "", outText = "";
  const modal = () => $("#jobs-modal");

  const state = j => (!j.done ? ["running", "⏳", L("çalışıyor", "running"), "info"] : j.exit === 0 ? ["ok", "✔", L("tamam", "done"), "ok"] : ["fail", "✖", L("hata", "failed"), "danger"]);
  const clock = iso => { const d = new Date(iso); return isNaN(d) ? "" : d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" }); };

  function paint() {
    const box = $("#jobs-list");
    if (!jobs.length) { setHtml(box, html`<p class="muted">${L("Bu oturumda henüz iş çalışmadı.", "No job has run in this session yet.")}</p>`); return; }
    setHtml(box, html`${jobs.map(j => {
      const [, ico, tr, cls] = state(j), open = openId === j.id;
      const what = j.action === "upgrade" ? `${L("paket kurulumu", "package install")} · ${j.package || ""}` : `${j.action}${j.package ? " · " + hooks.tabLabel(j.package) : ""}`;
      return html`<div class="batch-item"><div class="imp-row" data-id="${j.id}" role="button" tabindex="0" aria-expanded="${open}">
        <span class="imp-main"><b>${j.server || "—"}</b><span class="small muted mono">${what} · ${clock(j.started)}</span></span>
        <span class="badge ${cls}">${ico} ${tr}</span></div>${open && html`<pre class="batch-out">${outText}</pre>`}</div>`;
    })}`);
  }

  async function load() {
    try { const r = await hooks.json("/api/jobs"); jobs = r.jobs || []; running = r.running || ""; }
    catch (e) { $("#jobs-status").textContent = L("Liste alınamadı: ", "Could not load: ") + e.message; return; }
    $("#jobs-status").textContent = L(`${jobs.length} iş${running ? " · biri çalışıyor" : ""}`, `${jobs.length} job(s)${running ? " · one is running" : ""}`);
    if (openId) await fetchOut(openId);
    paint();
  }

  async function fetchOut(id) {
    try { const r = await hooks.json(`/api/run?id=${encodeURIComponent(id)}&from=0`); outText = (r.text || "").replace(/\r/g, "").trim().split("\n").slice(-60).join("\n") || L("(çıktı yok)", "(no output)"); }
    catch (e) { outText = L("Çıktı bellekte yok; ayrıntı için Günlükler'e bakın.", "Output is no longer in memory; see Logs."); }
  }

  async function onRow(e) {
    const row = e.target.closest("[data-id]"); if (!row) return;
    const id = row.dataset.id;
    if (openId === id) openId = ""; else { openId = id; await fetchOut(id); }
    paint();
  }

  function open() {
    openId = ""; outText = ""; modal().classList.add("open");
    $("#jobs-list").innerHTML = ""; load();
    clearInterval(timer); timer = setInterval(load, 3000); // çalışan iş varken durum canlı kalsın
    setTimeout(() => $("#jobs-close").focus(), 40);
  }
  function close() { clearInterval(timer); timer = 0; modal().classList.remove("open"); }

  function init(h) {
    hooks = h;
    $("#jobs-btn").addEventListener("click", open);
    $("#jobs-close").addEventListener("click", close);
    $("#jobs-x").addEventListener("click", close);
    $("#jobs-logs").addEventListener("click", () => { close(); hooks.goLogs(""); });
    modal().addEventListener("click", e => { if (e.target === modal()) close(); });
    $("#jobs-list").addEventListener("click", onRow);
    $("#jobs-list").addEventListener("keydown", e => { if ((e.key === "Enter" || e.key === " ") && e.target.matches("[data-id]")) { e.preventDefault(); e.target.click(); } });
    document.addEventListener("keydown", e => { if (e.key === "Escape" && modal().classList.contains("open")) close(); });
  }
  window.KeyLabJobs = { init, open };
})();
