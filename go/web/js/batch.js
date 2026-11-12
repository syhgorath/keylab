/* KeyLab — toplu işlem: seçili sunucularda AYNI salt okunur eylemi sırayla çalıştırır (Sunucular sayfası > Toplu işlem).
   Yalnız salt okunur, sudo'suz, onaysız eylemler sunulur; yazan/yıkıcı/sudo'lu eylemler toplu YAPILAMAZ. Karar sunucudadır
   (bkz. batch.go): burada listelenen seçenekler ve "uygun değil" gerekçeleri sunucunun cevabıdır, istemci hiçbir kapıyı atlamaz. */
(function () {
  "use strict";
  const $ = (s, r = document) => r.querySelector(s);
  const $$ = (s, r = document) => Array.from(r.querySelectorAll(s));
  const esc = s => String(s ?? "").replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  const isEn = () => window.KeyLabI18n && window.KeyLabI18n.getLang() === "en";
  const L = (tr, en) => (isEn() ? en : tr);
  let hooks = null, poll = 0, opts = [], optTimer = 0;

  const modal = () => $("#batch-modal");
  const STATE = {
    pending: ["bekliyor", "waiting", "plain", "·"], running: ["çalışıyor", "running", "info", "⏳"], ok: ["tamam", "done", "ok", "✔"],
    fail: ["hata", "failed", "danger", "✖"], skipped: ["atlandı", "skipped", "warn", "↷"], cancelled: ["iptal", "cancelled", "plain", "⏹"],
  };

  function open() { modal().classList.add("open"); }
  function close() { clearInterval(poll); poll = 0; clearTimeout(optTimer); modal().classList.remove("open"); }

  const picked = () => $$("#batch-targets input[data-n]:checked").map(c => c.dataset.n);

  /* ---- 1. adım: sunucu + eylem seçimi ---- */
  function showChoose() {
    clearInterval(poll); poll = 0;
    const names = hooks.targets();
    $("#batch-title").textContent = L("Toplu işlem", "Bulk action");
    $("#batch-sub").textContent = L("Seçili sunucularda aynı salt okunur eylem sırayla çalıştırılır.", "Runs the same read-only action on the selected servers, one after another.");
    $("#batch-go").hidden = false; $("#batch-go").disabled = true; $("#batch-cancel").hidden = true; $("#batch-close").textContent = L("Vazgeç", "Cancel");
    if (!names.length) { $("#batch-body").innerHTML = `<p class="muted">${esc(L("Listede sunucu yok (süzgeç ya da arama hepsini eledi).", "No servers in the list (the filter or search removed them all)."))}</p>`; $("#batch-status").textContent = ""; return; }
    $("#batch-body").innerHTML = `
      <div class="field"><label for="batch-act">${esc(L("Eylem", "Action"))}</label><select id="batch-act" aria-describedby="batch-note"></select>
        <div class="hint" id="batch-note"></div></div>
      <label class="small batch-opt"><input type="checkbox" id="batch-stop"> <span>${esc(L("İlk hatada dur (kalanlar çalıştırılmaz)", "Stop at the first failure (the rest are not run)"))}</span></label>
      <div class="row-flex batch-tools"><b>${esc(L("Hedef sunucular", "Target servers"))}</b><span class="spacer"></span>
        <button class="btn sm ghost" id="batch-all" type="button">${esc(L("Tümü", "All"))}</button><button class="btn sm ghost" id="batch-none" type="button">${esc(L("Hiçbiri", "None"))}</button></div>
      <div class="imp-rows" id="batch-targets">${names.map(n => `<label class="imp-row"><input type="checkbox" data-n="${esc(n)}" checked><span class="imp-main"><b>${esc(n)}</b></span></label>`).join("")}</div>`;
    $$("#batch-targets input").forEach(c => c.addEventListener("change", scheduleOptions));
    $("#batch-all").addEventListener("click", () => { $$("#batch-targets input").forEach(c => { c.checked = true; }); scheduleOptions(); });
    $("#batch-none").addEventListener("click", () => { $$("#batch-targets input").forEach(c => { c.checked = false; }); scheduleOptions(); });
    $("#batch-act").addEventListener("change", summary);
    loadOptions();
  }

  function scheduleOptions() { clearTimeout(optTimer); optTimer = setTimeout(loadOptions, 250); }

  async function loadOptions() {
    const sel = $("#batch-act"); if (!sel) return;
    const names = picked(), keep = sel.value;
    if (!names.length) { opts = []; sel.innerHTML = ""; $("#batch-go").disabled = true; $("#batch-status").textContent = L("Sunucu seçilmedi", "No server selected"); $("#batch-note").textContent = ""; return; }
    $("#batch-status").textContent = L("Uygun eylemler denetleniyor…", "Checking available actions…");
    try {
      const r = await hooks.json("/api/batch/options", { servers: names });
      opts = r.options || [];
    } catch (e) { opts = []; $("#batch-status").textContent = L("Eylemler alınamadı: ", "Could not load actions: ") + e.message; sel.innerHTML = ""; $("#batch-go").disabled = true; return; }
    sel.innerHTML = opts.map((o, i) => `<option value="${i}">${esc(hooks.tabLabel(o.package))} · ${esc(o.action)} (${o.ok}/${o.total})</option>`).join("");
    const i = opts.findIndex(o => `${o.package}/${o.action}` === keep);
    if (i >= 0) sel.value = String(i);
    summary();
  }

  function summary() {
    const o = opts[parseInt(($("#batch-act") || {}).value, 10)], n = picked().length;
    if (!o) { $("#batch-go").disabled = true; $("#batch-note").textContent = L("Seçili sunucularda toplu çalıştırılabilen salt okunur eylem yok.", "No read-only action can be run in bulk on the selected servers."); $("#batch-status").textContent = ""; return; }
    const skip = o.total - o.ok;
    $("#batch-note").textContent = L(`Salt okunur: sunucularda hiçbir şey değişmez. Yazan, sudo'lu ve onay isteyen eylemler burada sunulmaz. Her sunucu tek tek, menüdekiyle aynı motor ve kapılarla çalışır; sonuçlar günlüğe yazılır.`,
      `Read-only: nothing changes on the servers. Actions that write, need sudo or ask for confirmation are not offered here. Each server runs one by one through the same engine and gates as the menu; results are logged.`);
    $("#batch-status").textContent = L(`${o.ok} sunucuda çalışacak${skip ? `, ${skip} sunucu atlanacak (uygun değil)` : ""}`, `Will run on ${o.ok} server(s)${skip ? `, ${skip} will be skipped (not eligible)` : ""}`);
    $("#batch-go").disabled = o.ok === 0 || n === 0;
  }

  async function start() {
    const o = opts[parseInt($("#batch-act").value, 10)]; if (!o) return;
    $("#batch-go").disabled = true;
    try {
      await hooks.json("/api/batch", { package: o.package, action: o.action, servers: picked(), stopOnFail: $("#batch-stop").checked });
    } catch (e) { $("#batch-status").textContent = L("Başlatılamadı: ", "Could not start: ") + e.message; $("#batch-go").disabled = false; return; }
    showProgress();
  }

  /* ---- 2. adım: ilerleme ---- */
  let expanded = new Set(), outCache = {};
  function showProgress() {
    open(); expanded = new Set(); outCache = {};
    $("#batch-go").hidden = true; $("#batch-cancel").hidden = false; $("#batch-cancel").disabled = false; $("#batch-close").textContent = L("Küçült", "Minimize");
    $("#batch-body").innerHTML = `<div id="batch-sum" class="small muted"></div><div class="imp-rows" id="batch-rows"></div>`;
    $("#batch-rows").addEventListener("click", onRow);
    clearInterval(poll); poll = setInterval(tick, 1000); tick();
  }

  async function onRow(e) {
    const row = e.target.closest("[data-job]"); if (!row) return;
    const id = row.dataset.job, n = row.dataset.srv;
    if (expanded.has(n)) expanded.delete(n); else { expanded.add(n); await fetchOut(id, n); }
    tick(true);
  }
  async function fetchOut(id, n) {
    try { const r = await hooks.json(`/api/run?id=${encodeURIComponent(id)}&from=0`); outCache[n] = (r.text || "").trim().split("\n").slice(-40).join("\n") || L("(çıktı yok)", "(no output)"); }
    catch (e) { outCache[n] = L("Çıktı bellekte yok (en son 20 iş tutulur); ayrıntı için Günlükler'e bakın.", "Output is no longer in memory (only the last 20 jobs are kept); see Logs."); }
  }

  let lastBatch = null;
  function paint(b) {
    const c = { ok: 0, fail: 0, skipped: 0, cancelled: 0, running: 0, pending: 0 };
    b.items.forEach(i => { c[i.state] = (c[i.state] || 0) + 1; });
    const fin = c.ok + c.fail + c.skipped + c.cancelled;
    $("#batch-title").innerHTML = `<span class="mono">${esc(b.action)}</span> · ${esc(hooks.tabLabel(b.package))}`;
    $("#batch-sub").textContent = L(`Toplu işlem · ${b.items.length} sunucu`, `Bulk action · ${b.items.length} server(s)`);
    $("#batch-sum").textContent = L(`${fin}/${b.items.length} bitti · ${c.ok} tamam · ${c.fail} hata · ${c.skipped} atlandı${c.cancelled ? ` · ${c.cancelled} iptal` : ""}`, `${fin}/${b.items.length} finished · ${c.ok} done · ${c.fail} failed · ${c.skipped} skipped${c.cancelled ? ` · ${c.cancelled} cancelled` : ""}`);
    $("#batch-status").textContent = b.done ? (b.cancelled ? L("İptal edildi", "Cancelled") : L("Bitti", "Finished")) : L("Çalışıyor…", "Running…");
    $("#batch-rows").innerHTML = b.items.map(i => {
      const [tr, en, cls, ico] = STATE[i.state] || STATE.pending;
      const open = expanded.has(i.server);
      return `<div class="batch-item"><div class="imp-row${i.job ? "" : " done"}"${i.job ? ` data-job="${esc(i.job)}" data-srv="${esc(i.server)}" role="button" tabindex="0" aria-expanded="${open}"` : ""}>
        <span class="imp-main"><b>${esc(i.server)}</b>${i.reason ? `<span class="small muted">${esc(i.reason)}</span>` : ""}</span>
        <span class="badge ${cls}">${ico} ${esc(L(tr, en))}</span></div>${open ? `<pre class="batch-out">${esc(outCache[i.server] || "")}</pre>` : ""}</div>`;
    }).join("");
    $("#batch-cancel").hidden = b.done; $("#batch-close").textContent = b.done ? L("Kapat", "Close") : L("Küçült", "Minimize");
    renderBtn(b);
  }

  async function tick(local) {
    if (!local) {
      try { lastBatch = (await hooks.json("/api/batch")).batch; } catch (e) { $("#batch-status").textContent = L("Bağlantı koptu: ", "Connection lost: ") + e.message; return; }
    }
    const b = lastBatch; if (!b) return;
    paint(b);
    if (b.done) { clearInterval(poll); poll = 0; hooks.reload(); }
  }

  async function cancel() {
    $("#batch-cancel").disabled = true;
    try { await hooks.json("/api/batch/cancel", {}); } catch (e) { hooks.toast(e.message); }
  }

  /* ---- Sunucular sayfasındaki düğme ---- */
  function renderBtn(b) {
    const el = $("#batch-open"); if (!el) return;
    const run = b && !b.done;
    el.textContent = run ? L(`⏳ Toplu işlem (${b.items.filter(i => ["ok", "fail", "skipped", "cancelled"].includes(i.state)).length}/${b.items.length})`, `⏳ Bulk action (${b.items.filter(i => ["ok", "fail", "skipped", "cancelled"].includes(i.state)).length}/${b.items.length})`) : L("Toplu işlem…", "Bulk action…");
  }

  async function onButton() {
    let b = null;
    try { b = (await hooks.json("/api/batch")).batch; } catch (e) { /* sunucu yanıt vermedi: seçim ekranına düş */ }
    open();
    if (b && !b.done) { lastBatch = b; showProgress(); return; }
    showChoose();
  }

  // başka sayfadayken ya da yenilemeden sonra çalışan toplu işlemi düğmede göster
  async function sync() {
    try { const b = (await hooks.json("/api/batch")).batch; lastBatch = b; renderBtn(b); if (b && !b.done) setTimeout(sync, 2000); } catch (e) { /* sonraki turda */ }
  }

  function init(h) {
    hooks = h;
    $("#batch-open").addEventListener("click", onButton);
    $("#batch-go").addEventListener("click", start);
    $("#batch-cancel").addEventListener("click", cancel);
    $("#batch-close").addEventListener("click", close);
    $("#batch-x").addEventListener("click", close);
    modal().addEventListener("click", e => { if (e.target === modal()) close(); });
    document.addEventListener("keydown", e => { if (e.key === "Escape" && modal().classList.contains("open")) close(); });
    $("#batch-body").addEventListener("keydown", e => { if ((e.key === "Enter" || e.key === " ") && e.target.matches("[data-job]")) { e.preventDefault(); e.target.click(); } });
    sync();
  }
  window.KeyLabBatch = { init };
})();
