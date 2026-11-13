/* KeyLab — ssh config içe aktarma penceresi (Sunucu ekle > ~/.ssh/config'ten içe aktar). app.js'ten çıkarıldı (refaktör 2b), kod birebir.
   Yalnız takma ad gönderilir; adres/kullanıcı/port sunucuda dosyadan yeniden okunur ve doğrulanır. Var olan sunucu atlanır. */
import { $, $$, t, esc, json } from "../lib/util.js";

// hooks: { load } içe aktarma sonrası sunucu listesini yenilemek için
export function initSshImport({ load }) {
  // Yalnız takma ad gönderilir; adres/kullanıcı/port sunucuda dosyadan yeniden okunur ve doğrulanır. Var olan sunucu atlanır.
  const impModal = $("#import-modal"); let impFrom = null;
  const impClose = () => { if (!impModal.classList.contains("open")) return; impModal.classList.remove("open"); if (impFrom && impFrom.focus) impFrom.focus(); impFrom = null; };
  const impPicked = () => $$("#imp-list input[data-alias]:checked").map(c => c.dataset.alias);
  const impSync = () => { const n = impPicked().length; $("#imp-go").disabled = n === 0; $("#imp-status").textContent = n ? `${n} ${t("imp_selected", "seçili")}` : ""; };
  async function impOpen() {
    impFrom = document.activeElement; impModal.classList.add("open");
    const box = $("#imp-list"); $("#imp-go").hidden = false; $("#imp-go").disabled = true; $("#imp-status").textContent = "";
    box.textContent = t("imp_loading", "Taranıyor…");
    let r;
    try { r = await json("/api/ssh-config-hosts"); } catch (e) { box.textContent = t("imp_fail", "Hata:") + " " + e.message; return; }
    if (!r.exists) { box.innerHTML = '<div class="small muted">~/.ssh/config dosyası bulunamadı.</div>'; return; }
    if (!r.hosts.length) { box.innerHTML = '<div class="small muted">Dosyada içe aktarılabilir Host kaydı yok (joker karakterli kayıtlar ve Match/Include blokları desteklenmez).</div>'; return; }
    box.innerHTML = '<div class="imp-rows">' + r.hosts.map(h => {
      const meta = `${esc(h.host)}${h.user ? " · " + esc(h.user) : ""}${h.port ? " · :" + esc(h.port) : ""}`;
      return `<label class="imp-row${h.added ? " done" : ""}"><input type="checkbox" data-alias="${esc(h.alias)}"${h.added ? " disabled" : ""}><span class="imp-main"><b>${esc(h.alias)}</b>${h.name !== h.alias ? ` <span class="muted mono">→ ${esc(h.name)}</span>` : ""}<span class="small muted">${meta}</span></span>${h.added ? '<span class="badge plain">Ekli</span>' : ""}</label>`;
    }).join("") + "</div>";
    $$("#imp-list input[data-alias]").forEach(c => c.addEventListener("change", impSync));
    impSync();
  }
  async function impRun() {
    const aliases = impPicked(); if (!aliases.length) return;
    $("#imp-go").disabled = true;
    try {
      const r = await json("/api/ssh-config-import", { aliases });
      const c = { imported: 0, skipped: 0, error: 0 }; r.results.forEach(x => { c[x.status] = (c[x.status] || 0) + 1; });
      const errs = r.results.filter(x => x.status === "error").map(x => `${esc(x.alias)}: ${esc(x.error)}`);
      $("#imp-list").innerHTML = `<div class="notice"><span class="ico" aria-hidden="true">✔</span><div><b>${c.imported}</b> ${t("imp_done", "sunucu eklendi")}${c.skipped ? `, ${c.skipped} ${t("imp_skipped", "zaten vardı (atlandı)")}` : ""}${c.error ? `, ${c.error} ${t("imp_errors", "hata")}` : ""}.</div></div>` + (errs.length ? `<ul class="small sj-d2c171">${errs.map(e => `<li>${e}</li>`).join("")}</ul>` : "");
      $("#imp-status").textContent = ""; $("#imp-go").hidden = true;
      await load();
    } catch (e) { $("#imp-status").textContent = t("imp_fail", "Hata:") + " " + e.message; $("#imp-go").disabled = false; }
  }
  $("#imp-open").addEventListener("click", impOpen);
  $("#imp-go").addEventListener("click", impRun);
  ["#imp-x", "#imp-cancel"].forEach(id => $(id).addEventListener("click", impClose));
  impModal.addEventListener("click", e => { if (e.target === impModal) impClose(); });
  document.addEventListener("keydown", e => { if (e.key === "Escape") impClose(); });
  return { open: impOpen, close: impClose };
}
