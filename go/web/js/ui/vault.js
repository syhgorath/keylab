/* KeyLab — şifreli passphrase kasası arayüzü (API: bkz. vault_api.go; çekirdek: internal/vault).
   - Açılışta kasa dosyası var ve kilitliyse tam ekran GİRİŞ perdesi (#vault-lock): ana parola doğruysa kasadaki passphrase'ler özel ajana yüklenir.
   - Oturum ortasında sunucu 423 verirse (boşta kilit / başka sekmeden kilit) perde yeniden iner; açık pencereler ve çalışan işler yerinde kalır.
   - Üstbilgideki anahtar düğmesi kasayı kilitler; Ayarlar > Kasa kartı: oluştur, passphrase ekle/sil, ana parolayı değiştir.
   Bu GERÇEK bir kilittir (gizlilik perdesinden [privacy.js] farklı): ana parola olmadan kasadaki passphrase'ler ve ajandaki anahtarlar yoktur.
   Parolalar yalnız istek gövdesinde gider; alanlar her işlemden sonra temizlenir; yanıtlar sır içermez. */
import { $, t, toast } from "../lib/util.js";

const post = async (path, body) => {
  const r = await fetch(path, { method: "POST", headers: { "X-Requested-With": "sshpocket" }, body: JSON.stringify(body || {}) });
  let d = {}; try { d = await r.json(); } catch (e) { /* gövde yok */ }
  return { ok: r.ok, status: r.status, data: d };
};
const get = async () => { const r = await fetch("/api/vault", { headers: { "X-Requested-With": "sshpocket" } }); if (!r.ok) throw new Error(r.status); return r.json(); };
const fmt = (s, o) => Object.keys(o).reduce((a, k) => a.split("{" + k + "}").join(o[k]), s);

export function initVault({ getServers = () => [], reloadPage = () => location.reload(), reloadData = () => {}, setTimer = (f, ms) => setTimeout(f, ms) } = {}) {
  let st = { available: false }, startedLocked = false, curtainOn = false, from = null, waitTimer = 0;

  const setInert = on => {
    for (const el of Array.from(document.body.children)) {
      if (el.id === "vault-lock" || el.tagName === "SCRIPT") continue;
      if (on) el.setAttribute("inert", ""); else el.removeAttribute("inert");
    }
  };

  /* ---------------- giriş perdesi ---------------- */
  function showCurtain(msg) {
    const el = $("#vault-lock"); if (!el) return;
    if (!curtainOn) { from = document.activeElement; curtainOn = true; el.hidden = false; setInert(true); }
    $("#vault-lock-err").textContent = msg || "";
    $("#vault-lock-pw").value = "";
    $("#vault-lock-pw").focus();
  }
  function hideCurtain() {
    const el = $("#vault-lock"); if (!el || !curtainOn) return;
    curtainOn = false; el.hidden = true; setInert(false);
    if (window.KeyLabModalTrap) window.KeyLabModalTrap.sync();
    if (from && from.isConnected && from.focus) from.focus();
    from = null;
  }
  // base: bekleme mesajının önüne konan açıklama (ör. "Ana parola yanlış."); süre bitince ekranda yalnız o kalır
  function countdown(n, base = "") {
    clearTimeout(waitTimer);
    const btn = $("#vault-lock-go");
    if (n <= 0) { btn.disabled = false; $("#vault-lock-err").textContent = base; return; }
    btn.disabled = true;
    $("#vault-lock-err").textContent = (base ? base + " " : "") + fmt(t("vault_wait", "Çok fazla yanlış deneme; {n} sn bekleyin."), { n });
    waitTimer = setTimer(() => countdown(n - 1, base), 1000);
  }
  async function submitUnlock(e) {
    e.preventDefault();
    const pw = $("#vault-lock-pw").value; if (!pw) return;
    const btn = $("#vault-lock-go"); btn.disabled = true;
    let r;
    try { r = await post("/api/vault/unlock", { password: pw }); } catch (err) { r = { ok: false, status: 0, data: { error: String(err.message || err) } }; }
    $("#vault-lock-pw").value = "";
    if (r.ok) {
      const failed = r.data.failed || [];
      hideCurtain(); await refresh();
      if (startedLocked) { reloadPage(); return; } // sayfa kilitliyken yüklendi: eksik başlangıç istekleri için tam yenile
      reloadData();
      toast(fmt(t("vault_opened", "Kasa açıldı: {n} anahtar ajana yüklendi."), { n: (r.data.loaded || []).length }) + (failed.length ? " " + fmt(t("vault_failed", "Yüklenemeyen: {names}"), { names: failed.join(", ") }) : ""));
      return;
    }
    btn.disabled = false;
    if (r.status === 429) { countdown(r.data.retryAfter || 1); return; }
    const msg = r.status === 401 ? t("vault_wrong", "Ana parola yanlış.") : (r.data.error || t("vault_err", "Kasa açılamadı."));
    $("#vault-lock-err").textContent = msg;
    if (r.data.retryAfter > 0) countdown(r.data.retryAfter, msg);
    $("#vault-lock-pw").focus();
  }

  /* ---------------- durum ---------------- */
  async function refresh() {
    try { st = await get(); } catch (e) { st = { available: false }; }
    paint();
    return st;
  }
  function paint() {
    const card = $("#vault-card"); if (!card) return;
    card.hidden = !st.available;
    $("#vault-lock-btn").hidden = !(st.available && st.unlocked);
    if (!st.available) return;
    $("#vault-new").hidden = st.exists;
    $("#vault-open").hidden = !(st.exists && st.unlocked);
    $("#vault-state").textContent = !st.exists ? t("vault_s_none", "kasa yok") : st.unlocked ? t("vault_s_open", "açık") : t("vault_s_locked", "kilitli");
    if (!(st.exists && st.unlocked)) return;
    const ul = $("#vault-stored"); ul.textContent = "";
    for (const n of st.stored || []) {
      const li = document.createElement("li");
      const nm = document.createElement("span"); nm.className = "mono"; nm.textContent = n;
      const b = document.createElement("button"); b.type = "button"; b.className = "btn sm"; b.textContent = t("vault_rm", "Sil"); b.dataset.srv = n;
      li.append(nm, b); ul.append(li);
    }
    if (!(st.stored || []).length) { const li = document.createElement("li"); li.className = "muted small"; li.textContent = t("vault_none_stored", "Henüz passphrase eklenmedi."); ul.append(li); }
    const sel = $("#vault-srv"); const keep = sel.value; sel.textContent = "";
    for (const s of getServers()) { const o = document.createElement("option"); o.value = s.name || s; o.textContent = s.name || s; sel.append(o); }
    if (keep) sel.value = keep;
    $("#vault-idle").textContent = st.idleMin > 0 ? fmt(t("vault_idle_n", "{n} dk işlem yapılmazsa kilitlenir"), { n: st.idleMin }) : t("vault_idle_off", "otomatik kilit kapalı");
  }
  const msg = (text, bad) => { const el = $("#vault-msg"); el.textContent = text || ""; el.classList.toggle("bad", !!bad); };
  const errText = r => r.status === 401 ? t("vault_wrong", "Ana parola yanlış.") : r.status === 429 ? fmt(t("vault_wait", "Çok fazla yanlış deneme; {n} sn bekleyin."), { n: r.data.retryAfter || 1 }) : (r.data.error || t("vault_err", "İşlem başarısız."));

  async function lockNow() {
    const r = await post("/api/vault/lock");
    if (!r.ok) { toast(errText(r)); return; }
    await refresh(); startedLocked = false; showCurtain("");
  }

  /* ---------------- ayarlar kartı ---------------- */
  async function create() {
    const a = $("#vault-new-pw").value, b = $("#vault-new-pw2").value;
    if (a !== b) { msg(t("vault_mismatch", "Parolalar aynı değil."), true); return; }
    if ([...a].length < 8) { msg(t("vault_short", "Ana parola en az 8 karakter olmalı."), true); return; }
    const r = await post("/api/vault/create", { password: a });
    $("#vault-new-pw").value = ""; $("#vault-new-pw2").value = "";
    if (!r.ok) { msg(errText(r), true); return; }
    msg(t("vault_created", "Kasa oluşturuldu ve açık. Anahtar passphrase'lerini aşağıdan ekleyin."), false); await refresh();
  }
  async function add() {
    const server = $("#vault-srv").value, pp = $("#vault-pp").value;
    if (!server || !pp) { msg(t("vault_need", "Sunucu ve passphrase gerekli."), true); return; }
    const r = await post("/api/vault/passphrase", { server, passphrase: pp });
    $("#vault-pp").value = "";
    if (!r.ok) { msg(errText(r), true); return; }
    msg(fmt(t("vault_added", "{s}: passphrase kasaya eklendi ve anahtar ajana yüklendi."), { s: server }), false); await refresh();
  }
  async function remove(server) {
    const r = await post("/api/vault/passphrase/delete", { server });
    if (!r.ok) { msg(errText(r), true); return; }
    msg(fmt(t("vault_removed", "{s}: passphrase kasadan silindi."), { s: server }), false); await refresh();
  }
  async function change() {
    const o = $("#vault-old-pw").value, n = $("#vault-chg-pw").value, n2 = $("#vault-chg-pw2").value;
    if (n !== n2) { msg(t("vault_mismatch", "Parolalar aynı değil."), true); return; }
    if ([...n].length < 8) { msg(t("vault_short", "Ana parola en az 8 karakter olmalı."), true); return; }
    const r = await post("/api/vault/password", { password: o, newPassword: n });
    for (const id of ["#vault-old-pw", "#vault-chg-pw", "#vault-chg-pw2"]) $(id).value = "";
    if (!r.ok) { msg(errText(r), true); return; }
    msg(t("vault_changed", "Ana parola değiştirildi."), false);
  }

  /* ---------------- bağlama ---------------- */
  $("#vault-lock-form").addEventListener("submit", submitUnlock);
  $("#vault-lock-btn").addEventListener("click", lockNow);
  $("#vault-lock-now").addEventListener("click", lockNow);
  $("#vault-create").addEventListener("click", create);
  $("#vault-add").addEventListener("click", add);
  $("#vault-chg-go").addEventListener("click", change);
  window.addEventListener("keylab:homeloaded", paint); // sunucu listesi (home) kasa durumundan sonra yüklenebilir
  $("#vault-stored").addEventListener("click", e => { const b = e.target.closest("button[data-srv]"); if (b) remove(b.dataset.srv); });
  // gerçek kullanıcı girdisi (fare/klavye) boşta sayacını sıfırlar; en çok 30 sn'de bir bildirilir. Arka plan yoklamaları (GET) sayılmaz.
  let lastBeat = 0;
  const beat = () => {
    if (!(st.available && st.unlocked) || curtainOn) return;
    const n = Date.now(); if (n - lastBeat < 30000) return; lastBeat = n;
    post("/api/vault/touch").catch(() => {});
  };
  for (const ev of ["pointerdown", "keydown"]) window.addEventListener(ev, beat, { capture: true, passive: true });
  window.addEventListener("keylab:vaultlocked", () => { if (!curtainOn) { refresh(); showCurtain(t("vault_relocked", "Kasa kilitlendi (boşta kalma ya da elle). Devam etmek için ana parolayı yazın.")); } });

  return refresh().then(s => {
    if (s.available && s.exists && !s.unlocked) { startedLocked = true; showCurtain(""); }
    return s;
  });
}
