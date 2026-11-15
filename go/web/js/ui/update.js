/* KeyLab — uygulama içi güncelleme (API: GET /api/update/check, POST /api/update/apply, GET /api/update/status; bkz. update_api.go).
   Yalnız kullanıcı "Güncellemeyi denetle" deyince internete çıkılır (KeyLab kendiliğinden bakmaz). Yeni sürüm varsa GitHub sürüm notları gösterilir
   (release-publish.sh bunları CHANGELOG'dan koyar) ve — KeyLab.app olarak, yazılabilir konumdan çalışıyorsa — tek düğmeyle güncellenir: paket indirilir, imzası
   doğrulanır, .app değiştirilir, KeyLab yeniden açılır. Sürüm notları GitHub'dan gelen metindir: yalnız DOM düğümleriyle (textContent) çizilir, HTML olarak YORUMLANMAZ. */
import { $, t, toast } from "../lib/util.js";

const hdr = { "X-Requested-With": "sshpocket" };
const fmt = (s, o) => Object.keys(o).reduce((a, k) => a.split("{" + k + "}").join(o[k]), s);

// Sürüm notları (Markdown'un küçük bir alt kümesi): "### başlık", "- madde", **kalın**, `kod`; geri kalanı düz paragraf. Hepsi textContent.
export function renderNotes(box, text) {
  box.replaceChildren();
  let ul = null;
  const inline = (el, s) => {
    for (const part of s.split(/(\*\*[^*]+\*\*|`[^`]+`)/)) {
      if (!part) continue;
      if (part.startsWith("**") && part.endsWith("**") && part.length > 4) { const b = document.createElement("b"); b.textContent = part.slice(2, -2); el.append(b); }
      else if (part.startsWith("`") && part.endsWith("`") && part.length > 2) { const c = document.createElement("code"); c.textContent = part.slice(1, -1); el.append(c); }
      else el.append(document.createTextNode(part));
    }
  };
  for (const raw of String(text || "").split("\n")) {
    const line = raw.replace(/\s+$/, "");
    if (!line.trim()) { ul = null; continue; }
    let m;
    if ((m = /^#{1,4}\s+(.*)$/.exec(line))) { ul = null; const h = document.createElement("h5"); inline(h, m[1]); box.append(h); }
    else if ((m = /^[-*]\s+(.*)$/.exec(line))) { if (!ul) { ul = document.createElement("ul"); box.append(ul); } const li = document.createElement("li"); inline(li, m[1]); ul.append(li); }
    else { ul = null; const p = document.createElement("p"); inline(p, line); box.append(p); }
  }
}

const STEPS = { "indiriliyor": ["upd_s_dl", "Paket indiriliyor…"], "imza doğrulanıyor": ["upd_s_sig", "İmza doğrulanıyor…"], "açılıyor": ["upd_s_unzip", "Paket açılıyor…"],
  "doğrulanıyor": ["upd_s_check", "Paket denetleniyor…"], "yerleştiriliyor": ["upd_s_swap", "Yeni sürüm yerleştiriliyor…"] };
const WHY = {
  not_bundle: ["upd_why_bundle", "KeyLab.app olarak çalışmıyorsunuz (komut satırı ikilisi). Terminalde `keylab update` kullanın."],
  translocated: ["upd_why_transloc", "macOS uygulamayı karantina nedeniyle geçici bir konumdan çalıştırıyor, bu yüzden kendini güncelleyemez: KeyLab.app'i Uygulamalar'a (ya da USB'ye) taşıyıp oradan açın."],
  not_writable: ["upd_why_ro", "Uygulamanın bulunduğu klasöre yazılamıyor (USB yazma korumalı olabilir ya da izin yok)."],
};

export function initUpdate({ getVersion = () => "", openDlg, closeDlg, showView = () => {} } = {}) {
  let info = null, pollTimer = 0;
  const msg = (s, bad) => { const el = $("#appupd-state"); el.textContent = s || ""; el.classList.toggle("bad", !!bad); };

  function paintVersion() { const v = getVersion(); $("#appupd-cur").textContent = v ? "v" + v : ""; }

  function paint() {
    paintVersion();
    const res = $("#appupd-result"); res.hidden = !info;
    $("#appupd-badge").hidden = !(info && info.newer);
    if (!info) return;
    $("#appupd-line").textContent = info.newer ? fmt(t("upd_new", "Yeni sürüm var: v{l} (kurulu: v{c})"), { l: info.latest, c: info.current }) : fmt(t("upd_ok", "Güncelsiniz: v{c} (son sürüm v{l})"), { l: info.latest, c: info.current });
    $("#appupd-badge").textContent = "⬆ v" + info.latest;
    const nb = $("#appupd-notes"); nb.hidden = !info.notes;
    if (info.notes) { renderNotes(nb, info.notes); nb.setAttribute("aria-label", fmt(t("upd_notes", "v{l} sürüm notları"), { l: info.latest })); }
    $("#appupd-notes-h").hidden = !info.notes; $("#appupd-notes-h").textContent = fmt(t("upd_notes", "v{l} sürüm notları"), { l: info.latest });
    const apply = $("#appupd-apply"), why = $("#appupd-why");
    apply.hidden = !(info.newer && info.canApply); apply.disabled = false;
    const w = info.newer && !info.canApply ? WHY[info.reason] : null;
    why.hidden = !w; why.textContent = w ? t(w[0], w[1]) : "";
  }

  async function check() {
    const b = $("#appupd-check"); b.disabled = true; msg(t("upd_checking", "Denetleniyor…"));
    try {
      const r = await fetch("/api/update/check", { headers: hdr });
      if (!r.ok) { info = null; paint(); msg(r.status === 502 ? t("upd_offline", "Son sürüm öğrenilemedi (internet yok mu?).") : t("upd_err", "Denetlenemedi."), true); return; }
      info = await r.json(); msg(""); paint();
    } catch (e) { info = null; paint(); msg(t("upd_err", "Denetlenemedi."), true); }
    finally { b.disabled = false; }
  }

  function stopPoll() { clearInterval(pollTimer); pollTimer = 0; }
  function startPoll() {
    stopPoll();
    pollTimer = setInterval(async () => {
      try { const s = await (await fetch("/api/update/status", { headers: hdr })).json(); if (s.state === "running" && s.step) { const k = STEPS[s.step]; msg(k ? t(k[0], k[1]) : s.step); } } catch (e) { /* sonraki turda */ }
    }, 500);
  }

  async function doApply(force) {
    const b = $("#appupd-apply"); b.disabled = true; msg(t("upd_starting", "Güncelleme başlıyor…")); startPoll();
    let r, d = {};
    try { r = await fetch("/api/update/apply", { method: "POST", headers: hdr, body: JSON.stringify({ force: !!force }) }); d = await r.json().catch(() => ({})); }
    catch (e) { stopPoll(); msg(t("upd_apply_err", "Güncelleme başarısız."), true); b.disabled = false; return; }
    stopPoll();
    if (r.ok) { msg(fmt(t("upd_restarting", "Güncellendi: v{v}. KeyLab yeniden başlatılıyor…"), { v: d.version || info.latest })); b.hidden = true; return; }
    b.disabled = false;
    if (d.error === "running") { ask(d.jobs || 1); return; }
    const known = { busy: t("upd_busy", "Başka bir güncelleme sürüyor."), up_to_date: t("upd_uptodate", "Zaten güncelsiniz."), offline: t("upd_offline", "Son sürüm öğrenilemedi (internet yok mu?).") };
    const why = WHY[d.error];
    msg(known[d.error] || (why ? t(why[0], why[1]) : (d.message || t("upd_apply_err", "Güncelleme başarısız."))), true);
  }

  // onay: güncelleme KeyLab'ı yeniden başlatır (açık terminaller kapanır, kasa kilitlenir); çalışan iş varsa ayrıca uyarı
  function ask(running) {
    openDlg({
      title: t("upd_dlg_title", "Güncelle ve yeniden başlat"), sub: info ? "v" + info.current + " → v" + info.latest : "",
      body: (running ? `<p><b>${running} ${t("quit_running", "iş çalışıyor")}</b>: ${t("quit_running2", "çıkarsanız yarıda kesilir.")}</p>` : "") +
        `<p>${t("upd_dlg_body", "Paket indirilip imzası doğrulanacak, sonra KeyLab yeni sürümle yeniden açılacak. Açık terminaller kapanır ve şifreli kasa kilitlenir; sunucularınız, anahtarlarınız ve ayarlarınız korunur (eski sürüm yedeklenir).")}</p>`,
      ok: running ? t("upd_dlg_force", "Yine de güncelle") : t("upd_dlg_ok", "Güncelle"), okClass: running ? "danger" : "",
      onOk: () => { closeDlg(); doApply(running > 0); },
    });
  }

  $("#appupd-check").addEventListener("click", check);
  $("#appupd-apply").addEventListener("click", () => ask(0));
  $("#appupd-badge").addEventListener("click", () => { showView("settings"); const c = $("#appupd-card"); if (c && c.scrollIntoView) c.scrollIntoView({ block: "start" }); });
  paintVersion();
  return { check, paintVersion };
}
