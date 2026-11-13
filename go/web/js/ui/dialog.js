/* KeyLab — ortak iletişim penceresi (#dlg): başlık/gövde/Tamam-Vazgeç düğmeleri, Esc ile kapanma. Adres düzenleme, yeniden adlandırma, silme,
   paket ayrıntıları, Güncellemeler vb. kullanır. app.js'ten çıkarıldı (refaktör 5e), kod BİREBİR. Testler: go/webtest/ayarlar.test.js */
import { $ } from "../lib/util.js";

// → { dlg, openDlg({title, sub, body, ok, okClass, onOk, wide, noOk}), closeDlg() }
export function initDialog() {
  const dlg = $("#dlg");
  let dlgOk = null;
  function openDlg({ title, sub, body, ok, okClass, onOk, wide, noOk }) {
    $("#dlg-title").textContent = title; $("#dlg-sub").textContent = sub || ""; $("#dlg-body").innerHTML = body; $("#dlg-err").textContent = "";
    const b = $("#dlg-ok"); b.textContent = ok; b.className = "btn " + (okClass || "primary"); b.disabled = false; dlgOk = onOk;
    b.hidden = !!noOk; $("#dlg-cancel").textContent = noOk ? "Kapat" : "Vazgeç"; { const m = $(".modal", dlg); m.style.width = wide ? "min(860px,94vw)" : "min(580px,94vw)"; m.style.maxWidth = wide ? "860px" : ""; }
    dlg.classList.add("open");
  }
  const closeDlg = () => dlg.classList.remove("open");
  $("#dlg-x").addEventListener("click", closeDlg); $("#dlg-cancel").addEventListener("click", closeDlg);
  $("#dlg-ok").addEventListener("click", () => dlgOk && dlgOk());
  document.addEventListener("keydown", e => { if (e.key === "Escape") closeDlg(); });

  return { dlg, openDlg, closeDlg };
}
