/* KeyLab — Çıkış düğmesi: onay penceresi (#dlg) → POST /api/quit (terminalleri kapatır, USB'den çalışılıyorsa anahtarları ssh-agent'tan siler ve USB'yi
   çıkarır, uygulamayı durdurur) → "kapatıldı" perdesi (#quit-done). Çalışan iş varsa sunucu 409 "running" döner; ikinci onayla force=true gider.
   Keychain kayıtlarına dokunulmaz. Testler: go/webtest/quit.test.js */
import { $, esc, api, json, t } from "../lib/util.js";

// hooks: { openDlg, closeDlg }
export function initQuit(hooks) {
  const { openDlg, closeDlg } = hooks;

  function body(usb, running) {
    const parts = [];
    if (running) parts.push(`<p><b>${esc(String(running))} ${esc(t("quit_running", "iş çalışıyor"))}</b>: ${esc(t("quit_running2", "çıkarsanız yarıda kesilir."))}</p>`);
    parts.push(`<p>${esc(t("quit_what", "Açık terminaller kapanır, KeyLab durur."))}</p>`);
    if (usb) parts.push(`<p>${esc(t("quit_usb", "USB çıkarılır"))}: <span class="mono">${esc(usb)}</span> — ${esc(t("quit_usb2", "bu USB'deki anahtarlar ssh-agent'tan silinir."))}</p>`);
    parts.push(`<p class="small muted">${esc(t("quit_keychain", "Keychain kayıtlarına dokunulmaz."))}</p>`);
    return parts.join("");
  }

  function ask(usb, running) {
    openDlg({
      title: t("quit_title", "Çıkış"), sub: running ? t("quit_sub_force", "Çalışan iş var") : "",
      body: body(usb, running), ok: running ? t("quit_ok_force", "Yine de çık") : t("quit_ok", "Çıkış yap"), okClass: "danger",
      onOk: () => go(running > 0)
    });
  }

  function done(d) {
    closeDlg();
    const o = $("#quit-done"); if (!o) return;
    let line = t("quit_done_plain", "KeyLab durduruldu. Bu pencereyi kapatabilirsiniz.");
    if (d.usb) line = d.deferred ? `${t("quit_done_deferred", "USB uygulama kapandıktan hemen sonra çıkarılacak")}: ${d.usb}. ${line}` : d.ejected ? `${t("quit_done_usb", "USB çıkarıldı")}: ${d.usb}. ${line}` : `${t("quit_done_nousb", "USB çıkarılamadı, elle çıkarın")}: ${d.usb}. ${line}`;
    $("#quit-done-text").textContent = line;
    $("#quit-done-log").textContent = d.log || "";
    o.hidden = false;
    try { window.close(); } catch (e) { /* uygulama penceresi dışında kapanmayabilir */ }
  }

  async function go(force) {
    const b = $("#dlg-ok"), err = $("#dlg-err");
    b.disabled = true;
    try { done(await json("/api/quit", { force })); }
    catch (e) {
      let j = null; try { j = JSON.parse(e.message); } catch (x) { /* düz metin hata */ }
      if (j && j.error === "running") { ask(await usbName(), j.jobs || 1); return; }
      b.disabled = false; err.textContent = e.message || String(e);
    }
  }

  async function usbName() { try { return (await json("/api/about")).usb || ""; } catch (e) { return ""; } }

  $("#quit-btn").addEventListener("click", async () => ask(await usbName(), 0));
  return { ask };
}
