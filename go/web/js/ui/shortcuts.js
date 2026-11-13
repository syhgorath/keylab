/* KeyLab — klavye kısayolları penceresi (üst çubuktaki "?" düğmesi ve "?" tuşu). app.js'ten çıkarıldı (refaktör 2b), kod birebir. */
import { $ } from "../lib/util.js";

export function initShortcuts() {
  const scModal = $("#shortcuts-modal"); let scFrom = null;
  function openShortcuts() { scFrom = document.activeElement; scModal.classList.add("open"); $("#sc-close").focus(); }
  function closeShortcuts() { if (!scModal.classList.contains("open")) return; scModal.classList.remove("open"); if (scFrom && scFrom.focus) scFrom.focus(); scFrom = null; }
  $("#shortcuts-btn").addEventListener("click", openShortcuts);
  $("#sc-close").addEventListener("click", closeShortcuts);
  $("#sc-x").addEventListener("click", closeShortcuts);
  scModal.addEventListener("click", e => { if (e.target === scModal) closeShortcuts(); });
  document.addEventListener("keydown", e => {
    if (e.key !== "?" || e.ctrlKey || e.metaKey || e.altKey) return;
    if (/input|textarea|select/i.test(document.activeElement.tagName) || document.activeElement.isContentEditable) return;
    if (document.querySelector(".modal-back.open:not(#shortcuts-modal)")) return; // başka pencere açıkken açma
    e.preventDefault(); scModal.classList.contains("open") ? closeShortcuts() : openShortcuts();
  });
  return { open: openShortcuts, close: closeShortcuts };
}
