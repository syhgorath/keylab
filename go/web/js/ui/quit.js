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

  /* ---------------- canlı adımlar ---------------- */
  // Sunucu her adımı kaydeder (GET /api/quit/status); POST /api/quit adımlar bitene dek döner. Süreç "Kapat" (POST /api/quit/finish) ile durur.
  const ICON = { running: "⏳", done: "✔", skipped: "–", deferred: "📤", error: "⚠️", info: "ℹ️" };
  const STEP = {
    "terms.running": ["quit_s_terms_run", "Açık terminaller kapatılıyor…"], "terms.done": ["quit_s_terms_done", "{n} terminal oturumu kapatıldı"], "terms.skipped": ["quit_s_terms_skip", "Açık terminal yoktu"],
    "vault.running": ["quit_s_vault_run", "Şifreli kasa kilitleniyor…"], "vault.done": ["quit_s_vault_done", "Kasa kilitlendi; bellekteki anahtar ve passphrase'ler sıfırlandı"],
    "usb.running": ["quit_s_usb_run", "USB'deki anahtarlar ajandan siliniyor ve USB çıkarılıyor…"], "usb.done": ["quit_s_usb_done", "USB çıkarıldı: {name}"],
    "usb.deferred": ["quit_s_usb_def", "USB, uygulama kapandıktan hemen sonra çıkarılacak: {name}"], "usb.error": ["quit_s_usb_err", "USB çıkarılamadı, elle çıkarın: {name}"],
    "agent.running": ["quit_s_agent_run", "Özel ssh-agent durduruluyor…"], "agent.done": ["quit_s_agent_done", "Özel ssh-agent durduruldu; anahtarlar bellekten silindi"],
    "tmp.running": ["quit_s_tmp_run", "Geçici klasör siliniyor…"], "tmp.done": ["quit_s_tmp_done", "Geçici klasör silindi (Python ortamı, tarayıcı profili)"],
    "keychain.running": ["quit_s_kc_run", "Keychain denetleniyor…"], "keychain.info.guest": ["quit_s_kc_guest", "Bu bilgisayarın Keychain'i ve ssh-agent'ı hiç kullanılmadı"],
    "keychain.info.kept": ["quit_s_kc_kept", "Keychain kayıtlarına dokunulmadı (bu Mac'te kalır)"],
  };
  const stepText = s => {
    const k = STEP[`${s.id}.${s.state}.${s.detail}`] || STEP[`${s.id}.${s.state}`]; if (!k) return `${s.id}: ${s.state}`;
    return t(k[0], k[1]).split("{n}").join(String(s.n || 0)).split("{name}").join(s.detail || "");
  };
  function renderSteps(steps) {
    const ul = $("#quit-steps"); if (!ul) return;
    ul.replaceChildren(...(steps || []).map(s => {
      const li = document.createElement("li"); li.dataset.state = s.state;
      const ic = document.createElement("span"); ic.className = "quit-ico"; ic.setAttribute("aria-hidden", "true"); ic.textContent = ICON[s.state] || "•";
      const tx = document.createElement("span"); tx.textContent = stepText(s);
      li.append(ic, tx); return li;
    }));
  }
  let pollTimer = 0;
  function showCurtain() {
    const o = $("#quit-done"); if (!o || !o.hidden) return;
    closeDlg(); o.hidden = false;
    $("#quit-done-title").textContent = t("quit_closing", "KeyLab kapanıyor…"); $("#quit-finish").hidden = true;
  }
  function stopPoll() { clearInterval(pollTimer); pollTimer = 0; }
  function startPoll() {
    stopPoll();
    pollTimer = setInterval(async () => {
      try { const s = await json("/api/quit/status"); if (s.state) { showCurtain(); renderSteps(s.steps); } } catch (e) { /* sonraki turda */ }
    }, 250);
  }

  function done(d) {
    stopPoll(); showCurtain();
    const o = $("#quit-done"); if (!o) return;
    renderSteps(d.steps);
    $("#quit-done-title").textContent = t("quit_ready", "Kapatabilirsiniz");
    const fin = $("#quit-finish"); fin.hidden = false; fin.disabled = false; fin.focus();
    let line = t("quit_done_plain", "Temizlik tamam. Kapat'a basınca KeyLab durur.");
    if (d.usb) line = d.deferred ? `${t("quit_done_deferred", "USB uygulama kapandıktan hemen sonra çıkarılacak")}: ${d.usb}. ${line}` : d.ejected ? `${t("quit_done_usb", "USB çıkarıldı")}: ${d.usb}. ${line}` : `${t("quit_done_nousb", "USB çıkarılamadı, elle çıkarın")}: ${d.usb}. ${line}`;
    $("#quit-done-text").textContent = line;
    $("#quit-done-log").textContent = d.log || "";
    // taşınabilir mod: temizlenenler ve bu bilgisayarda kalanlar (kodlar sunucudan gelir; metinler burada çevrilir)
    const rep = d.report, box = $("#quit-done-report");
    if (box) {
      box.hidden = !rep;
      if (rep) {
        const fill = (sel, codes, names) => { const ul = $(sel); ul.replaceChildren(...(codes || []).map(c => { const li = document.createElement("li"); li.textContent = names[c] || c; return li; })); };
        fill("#quit-done-cleaned", rep.cleaned, {
          terms: t("quit_c_terms", "Açık terminaller kapatıldı"), agent: t("quit_c_agent", "Özel ssh-agent sonlandırıldı (anahtarlar bellekten silindi)"),
          tmp: t("quit_c_tmp", "Geçici klasör silindi (Python ortamı, tarayıcı profili)")
        });
        fill("#quit-done-remains", rep.remains, {
          macos: t("quit_r_macos", "macOS'un kendi izleri (Spotlight, son kullanılanlar, sistem günlükleri)"), keychain: t("quit_r_keychain", "Keychain kayıtları (passphrase, knock dizisi)"),
          user_agent: t("quit_r_agent", "Kendi ssh-agent'ınıza eklenen anahtarlar")
        });
      }
    }
    o.hidden = false;
  }

  // "Kapat": süreç durur (uygulama penceresi de kapanır); tarayıcı sekmesinde pencere kapatılamazsa kullanıcıya bildirilir
  async function finish() {
    const b = $("#quit-finish"); b.disabled = true;
    try { await api("/api/quit/finish", {}); } catch (e) { /* süreç zaten kapanmış olabilir */ }
    $("#quit-done-title").textContent = t("quit_done_title", "KeyLab kapatıldı"); b.hidden = true;
    $("#quit-done-text").textContent = t("quit_stopped", "KeyLab durduruldu. Bu pencereyi kapatabilirsiniz.");
    try { window.close(); } catch (e) { /* uygulama penceresi dışında kapanmayabilir */ }
  }

  async function go(force) {
    const b = $("#dlg-ok"), err = $("#dlg-err");
    b.disabled = true; startPoll();
    try { done(await json("/api/quit", { force })); }
    catch (e) {
      stopPoll();
      let j = null; try { j = JSON.parse(e.message); } catch (x) { /* düz metin hata */ }
      if (j && j.error === "running") { ask(await usbName(), j.jobs || 1); return; }
      b.disabled = false; err.textContent = e.message || String(e);
    }
  }

  async function usbName() { try { return (await json("/api/about")).usb || ""; } catch (e) { return ""; } }

  $("#quit-finish").addEventListener("click", finish);
  $("#quit-btn").addEventListener("click", async () => ask(await usbName(), 0));
  return { ask };
}
