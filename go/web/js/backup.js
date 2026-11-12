/* KeyLab — envanter yedeği: dışa aktar / içe aktar (Ayarlar > Envanter yedeği).
   Dışa aktarılan dosya SIR içermez (private key, knock dizisi, parola/token adlı değişkenler yok) ama sunucu adreslerini içerir.
   İçe aktarma iki adımlıdır: önce ÖNİZLEME (diske hiçbir şey yazılmaz), sonra yalnız seçilenler uygulanır. Var olan sunucular
   varsayılan olarak SEÇİLİ DEĞİLDİR. Doğrulama sunucudadır (bkz. backup.go); burada yalnız gösterilir. */
(function () {
  "use strict";
  const $ = (s, r = document) => r.querySelector(s);
  const $$ = (s, r = document) => Array.from(r.querySelectorAll(s));
  const esc = s => String(s ?? "").replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  const isEn = () => window.KeyLabI18n && window.KeyLabI18n.getLang() === "en";
  const L = (tr, en) => (isEn() ? en : tr);
  const MAX = 2 * 1024 * 1024;
  let hooks = null, pending = null; // pending: ayrıştırılmış yedek dosyası

  function download(name, text) {
    const a = document.createElement("a"), url = URL.createObjectURL(new Blob([text], { type: "application/json;charset=utf-8" }));
    a.href = url; a.download = name; document.body.appendChild(a); a.click(); a.remove(); setTimeout(() => URL.revokeObjectURL(url), 2000);
  }

  async function doExport() {
    const btn = $("#bk-export"); btn.disabled = true;
    try {
      const r = await hooks.json("/api/backup-export", {});
      download(`keylab-envanter-${new Date().toISOString().slice(0, 10)}.json`, JSON.stringify(r.file, null, 2));
      hooks.toast(L(`${r.file.servers.length} sunucu dışa aktarıldı`, `${r.file.servers.length} server(s) exported`) + (r.omittedSecrets ? L(` · ${r.omittedSecrets} gizli görünen değişken dışarıda bırakıldı`, ` · ${r.omittedSecrets} secret-looking variable(s) left out`) : ""));
    } catch (e) { hooks.toast(L("Dışa aktarılamadı: ", "Export failed: ") + e.message); }
    btn.disabled = false;
  }

  const ACTION = { create: ["Yeni", "New", "ok"], update: ["Üzerine yazar", "Overwrites", "warn"], same: ["Aynı", "Same", "plain"], invalid: ["Geçersiz", "Invalid", "fail"] };

  function renderPlan(rows) {
    const box = $("#bk-result");
    const sel = r => r.action === "create";
    box.innerHTML = `<div class="imp-rows">${rows.map(r => {
      const [tr, en, cls] = ACTION[r.action] || ACTION.invalid;
      const can = r.action === "create" || r.action === "update";
      const detail = r.action === "update" ? `${r.addrChanged ? L("adres değişir", "address changes") : ""}${r.addrChanged && r.varsChanged ? " · " : ""}${r.varsChanged ? L(`${r.varsChanged} değişken`, `${r.varsChanged} variable(s)`) : ""}`
        : r.action === "create" ? (r.varsChanged ? L(`${r.varsChanged} değişken`, `${r.varsChanged} variable(s)`) : "") : "";
      return `<label class="imp-row${can ? "" : " done"}"><input type="checkbox" data-n="${esc(r.name)}"${can ? "" : " disabled"}${sel(r) ? " checked" : ""}>
        <span class="imp-main"><b>${esc(r.name)}</b>${detail ? `<span class="small muted">${esc(detail)}</span>` : ""}${(r.notes || []).map(n => `<span class="small muted">• ${esc(n)}</span>`).join("")}</span>
        <span class="badge ${cls}">${esc(L(tr, en))}</span></label>`;
    }).join("")}</div>`;
    const sync = () => { const n = $$("#bk-result input[data-n]:checked").length; $("#bk-apply").disabled = n === 0; $("#bk-status").textContent = n ? `${n} ${L("seçili", "selected")}` : ""; };
    $$("#bk-result input[data-n]").forEach(c => c.addEventListener("change", sync));
    $("#bk-apply").hidden = false; sync();
  }

  async function onFile(file) {
    const box = $("#bk-result"); $("#bk-apply").hidden = true; $("#bk-status").textContent = "";
    if (!file) return;
    if (file.size > MAX) { box.textContent = L("Dosya çok büyük (en fazla 2 MB).", "File is too large (2 MB max)."); return; }
    let parsed;
    try { parsed = JSON.parse(await file.text()); } catch (e) { box.textContent = L("Dosya geçerli JSON değil.", "The file is not valid JSON."); return; }
    pending = parsed; box.textContent = L("Önizleme hazırlanıyor…", "Preparing the preview…");
    try {
      const r = await hooks.json("/api/backup-import", { file: parsed, apply: false, pick: [] });
      renderPlan(r.results);
    } catch (e) { box.textContent = L("Yedek okunamadı: ", "Could not read the backup: ") + e.message; pending = null; }
  }

  async function apply() {
    const pick = $$("#bk-result input[data-n]:checked").map(c => c.dataset.n); if (!pick.length || !pending) return;
    const overwrites = pick.filter(n => $(`#bk-result input[data-n="${CSS.escape(n)}"]`).closest(".imp-row").querySelector(".badge.warn"));
    if (overwrites.length && !confirm(L(`${overwrites.length} mevcut sunucunun adresi/değişkenleri yedekteki değerlerle DEĞİŞECEK:\n\n${overwrites.join(", ")}\n\nDevam edilsin mi?`, `${overwrites.length} existing server(s) will be CHANGED to the backup's values:\n\n${overwrites.join(", ")}\n\nContinue?`))) return;
    $("#bk-apply").disabled = true;
    try {
      const r = await hooks.json("/api/backup-import", { file: pending, apply: true, pick });
      const ok = r.results.filter(x => x.status === "ok").length, bad = r.results.filter(x => x.status === "error" && x.action !== "invalid"), inv = r.results.filter(x => x.action === "invalid").length;
      $("#bk-result").innerHTML = `<div class="notice"><span class="ico" aria-hidden="true">✔</span><div><b>${ok}</b> ${esc(L("sunucu uygulandı", "server(s) applied"))}${bad.length ? `, ${bad.length} ${esc(L("hata", "error(s)"))}` : ""}${inv ? `, ${inv} ${esc(L("geçersiz kayıt atlandı", "invalid entr(ies) skipped"))}` : ""}.</div></div>`
        + `<ul class="small sj-d2c171">${r.results.filter(x => (x.notes || []).length && (x.status === "ok" || x.status === "error")).map(x => `<li><b>${esc(x.name)}</b>: ${esc(x.notes.join(" · "))}</li>`).join("")}</ul>`;
      $("#bk-apply").hidden = true; $("#bk-status").textContent = ""; pending = null; $("#bk-file").value = "";
      await hooks.reload();
    } catch (e) { $("#bk-status").textContent = L("Hata: ", "Error: ") + e.message; $("#bk-apply").disabled = false; }
  }

  function init(h) {
    hooks = h;
    $("#bk-export").addEventListener("click", doExport);
    $("#bk-file").addEventListener("change", e => onFile(e.target.files[0]));
    $("#bk-apply").addEventListener("click", apply);
  }
  window.KeyLabBackup = { init };
})();
