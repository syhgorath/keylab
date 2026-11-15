/* KeyLab — Python ön denetimi (API: GET /api/python?package=…, bkz. python_api.go). Python/Ansible gerektiren bir paket eylemi başlamadan ÖNCE bu
   bilgisayarda uygun Python var mı sorulur; yoksa eylem terminalde hata verene kadar beklenmez, ne kurulacağını söyleyen bir pencere gösterilir.
   KeyLab Python'u ve Ansible'ı kendi içinde taşımaz (boyut ve lisans yükü) ve kendisi KURMAZ: kullanıcı kurar, "Yeniden denetle" der. */
import { $, esc, json, t } from "../lib/util.js";

const fmt = (s, o) => Object.keys(o).reduce((a, k) => a.split("{" + k + "}").join(o[k]), s);

// → checkPython(paket, devam): Promise<boolean> — true: devam edilebilir (gerekmiyor / var / denetlenemedi); false: pencere gösterildi, eylem başlatılmaz;
//   kullanıcı Python'u kurup "Yeniden denetle" derse pencere kapanır ve devam() çağrılır (eylem kaldığı yerden açılır).
export function initPython({ openDlg, closeDlg }) {
  async function status(pkg) {
    try { return await json("/api/python?package=" + encodeURIComponent(pkg)); }
    catch { return null; } // denetlenemezse engel olma: eylem kendi hatasını verir
  }
  function show(st, retry) {
    const found = (st.found || []).length ? `<p>${esc(fmt(t("py_found", "Bulunan ama uymayan sürümler: {v}."), { v: st.found.join(", ") }))}</p>` : `<p>${esc(t("py_none", "Bu bilgisayarda Python bulunamadı."))}</p>`;
    const stub = st.stub ? `<p class="small muted">${esc(t("py_stub", "/usr/bin/python3 yalnızca Command Line Tools kurulunca gerçek Python olur; KeyLab onu çalıştırmaz."))}</p>` : "";
    openDlg({
      title: t("py_title", "Önce Python kurmalısınız"), sub: fmt(t("py_sub", "Bu paket Ansible kullanır; Python {r} gerekir"), { r: st.range }),
      body: `${found}${stub}<ol><li>${esc(t("py_step1", "python.org/downloads adresinden macOS yükleyicisini indirip kurun"))} <span class="muted">${esc(t("py_step1b", "ya da Homebrew varsa:"))}</span> <code>brew install python@3.13</code></li>
        <li>${esc(t("py_step2", "Kurduktan sonra aşağıdan \"Yeniden denetle\"ye basın."))}</li></ol>
        <p class="small muted">${esc(t("py_why", "KeyLab Python'u ve Ansible'ı kendi içinde taşımaz (boyut ve lisans yükü nedeniyle) ve kendisi kurmaz. Sunucu yönetimi, terminal ve anahtarlar Python olmadan çalışır."))}</p>`,
      ok: t("py_retry", "Yeniden denetle"), onOk: retry,
    });
  }
  return async function checkPython(pkg, proceed) {
    if (!pkg) return true;
    const st = await status(pkg);
    if (!st || st.ok) return true;
    show(st, async () => { const b = $("#dlg-ok"); b.disabled = true; const s2 = await status(pkg); b.disabled = false; if (!s2 || s2.ok) { closeDlg(); if (proceed) proceed(); } else { $("#dlg-err").textContent = t("py_still", "Hâlâ uygun Python bulunamadı."); } });
    return false;
  };
}
