/* KeyLab — alt çubuk (dock): pocketui'deki "Görevler & Son İşlemler" çubuğunun karşılığı.
   Solda görev sekmesi (sayaç), açık terminal oturumu çipleri (js/terminal.js doldurur) ve çalışan iş çipi; sağda aç/kapat.
   Açılınca son işlemler tablosu: her sunucunun components.log kaydı (kalıcı geçmiş, sunucu başına 8) + çalışan iş. Salt okunur:
   iş başlatmaz/durdurmaz. Tablo satırından yalnız o sunucu+paketin SON çalıştırma günlüğü açılır (eski satırlarda günlük düğmesi yok). */
(function () {
  "use strict";
  const $ = (s, r = document) => r.querySelector(s);
  const { html, raw, setHtml } = window.KeyLabHtml;
  const isEn = () => window.KeyLabI18n && window.KeyLabI18n.getLang() === "en";
  const L = (tr, en) => (isEn() ? en : tr);
  const MAX_ROWS = 100;
  let hooks = null, open = false, running = [], timer = 0; // running: çalışan işlerin listesi (en yeni başta)

  try { open = localStorage.getItem("keylab.dock") === "open"; } catch (e) { /* depolama kapalı olabilir */ }

  const ok = r => r === "ok";
  const agoTxt = iso => { const x = hooks.ago(iso); return x === "şimdi" ? L("şimdi", "just now") : x + " " + L("önce", "ago"); };

  function rows() {
    const home = hooks.getHome();
    const all = (home.servers || []).flatMap(s => (s.recent || []).map(a => ({ ...a, server: s.name })))
      .sort((a, b) => Date.parse(b.time) - Date.parse(a.time));
    const seen = new Set();
    return all.slice(0, MAX_ROWS).map(a => { const k = a.server + "\u0000" + a.package, first = !seen.has(k); seen.add(k); return { ...a, newest: first }; });
  }

  function paint() {
    const dock = $("#wb-dock"), list = rows();
    dock.classList.toggle("collapsed", !open);
    document.body.classList.toggle("dock-open", open);
    $("#dock-tab").setAttribute("aria-expanded", String(open));
    $("#dock-toggle").textContent = open ? "▾" : "▴";
    $("#dock-body").hidden = !open;
    $("#dock-count").textContent = String(list.length + running.length);
    $("#dock-hint").textContent = open ? "" : L("Son işlemleri görmek için açın", "Open to see recent actions");

    setHtml($("#dock-jobs"), html`${running.map(r => html`<button class="dock-job" data-job="${r.id}" type="button" title="${L("Çalışan işin penceresini aç", "Open the running job's window")}"><span class="spinner" aria-hidden="true"></span> ${r.server}: ${r.action}</button>`)}`);

    if (!open) return;
    const live = running.map(r => html`<tr class="dock-row running"><td>${L("şimdi", "now")}</td><td class="mono"><b>${r.server}</b></td><td class="mono">${hooks.tabLabel(r.package)}</td><td class="mono">${r.action}</td><td><span class="badge info">⏳ ${L("çalışıyor", "running")}</span></td><td class="dock-act"><button class="btn sm ghost" data-job="${r.id}">${L("Aç", "Open")}</button></td></tr>`);
    const body = list.map(a => html`<tr class="dock-row${ok(a.result) ? "" : " is-fail"}">
        <td title="${hooks.localTime(a.time)}">${agoTxt(a.time)}</td>
        <td class="mono"><b>${a.server}</b></td><td class="mono">${a.package} ${a.version}</td><td class="mono">${a.action}</td>
        <td><span class="badge ${ok(a.result) ? "ok" : "danger"}">${ok(a.result) ? "✔ " + L("başarılı", "ok") : "✖ " + L("başarısız", "failed")}</span></td>
        <td class="dock-act">${a.newest ? html`<button class="btn sm ghost" data-log="${a.package}" data-srv="${a.server}" title="${L("Bu paketin bu sunucudaki son çalıştırma günlüğü", "This package's last run log on this server")}">${L("Günlük", "Log")}</button>` : raw("")}</td></tr>`);
    setHtml($("#dock-rows"), html`${live}${body}`);
    $("#dock-empty").hidden = list.length > 0 || running.length > 0;
  }

  async function poll() {
    try {
      const r = await hooks.json("/api/jobs");
      const ids = r.runningIds || (r.running ? [r.running] : []);
      const now = ids.map(id => (r.jobs || []).find(x => x.id === id)).filter(j => j && !j.done).map(j => ({ id: j.id, server: j.server, package: j.package, action: j.action }));
      const finished = running.some(o => !now.some(n => n.id === o.id)); // bir iş bitti: yeni kayıt tabloya girsin
      running = now;
      if (finished) await hooks.reload();
      paint();
    } catch (e) { /* sonraki turda */ }
  }

  function setOpen(v) {
    open = v; try { localStorage.setItem("keylab.dock", open ? "open" : "closed"); } catch (e) { /* önemli değil */ }
    paint(); if (v) poll();
  }

  function init(h) {
    hooks = h;
    $("#dock-tab").addEventListener("click", () => setOpen(!open));
    $("#dock-toggle").addEventListener("click", () => setOpen(!open));
    $("#dock-body").addEventListener("click", e => {
      const b = e.target.closest("[data-log]");
      if (b) { hooks.openLastLog(b.dataset.log, b.dataset.srv); return; }
      const j = e.target.closest("[data-job]"); if (j) hooks.openJob(j.dataset.job);
    });
    $("#dock-jobs").addEventListener("click", e => { const j = e.target.closest("[data-job]"); if (j) hooks.openJob(j.dataset.job); });
    window.addEventListener("keylab:langchange", paint);
    poll(); timer = setInterval(poll, 3000);
    paint();
  }
  window.KeyLabDock = { init, paint, toggle: () => setOpen(!open), isOpen: () => open };
})();
