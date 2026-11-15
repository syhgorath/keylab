/* KeyLab — "Bu Mac'e güveniyorum" ayarı (Ayarlar kartı; API: GET/POST /api/trusted-mac, bkz. trustedmac.go).
   Taşınabilir (USB) kullanımda varsayılan misafir mod: özel ssh-agent, Keychain yok. Güvendiğiniz kendi Mac'inizde bu işaret açılırsa bir sonraki
   açılıştan itibaren bu bilgisayarın Keychain'i ve ssh-agent'ı kullanılır (passphrase Keychain'de durur, "tak diye girer"). İşaret USB'de değil BU
   bilgisayarda tutulur: USB başka bir bilgisayara takılınca o bilgisayar güvenilir sayılmaz. Karar açılışta verildiği için değişiklik yeniden
   başlatınca geçerli olur. Şifreli kasa yalnız misafir modda vardır (kasa kartı Keychain modunda kendiliğinden gizlenir). */
import { $, t, toast } from "../lib/util.js";

const hdr = { "X-Requested-With": "sshpocket" };

export function initTrustedMac() {
  let st = { portable: false };
  const card = $("#tm-card"); if (!card) return Promise.resolve(st);

  function paint() {
    card.hidden = !st.portable;
    if (!st.portable) return;
    const keychain = st.active === "keychain";
    $("#tm-active").textContent = keychain ? t("tm_active_keychain", "Şu an: bu Mac'in Keychain'i ve ssh-agent'ı kullanılıyor.") : t("tm_active_guest", "Şu an: misafir mod (özel ssh-agent, Keychain kullanılmıyor).");
    $("#tm-state").textContent = st.trusted ? t("tm_s_on", "güvenilir") : t("tm_s_off", "misafir");
    const b = $("#tm-toggle");
    b.textContent = st.trusted ? t("tm_off", "Bu Mac'e artık güvenme") : t("tm_on", "Bu Mac'e güveniyorum: Keychain'i kullan");
    b.setAttribute("aria-pressed", String(!!st.trusted));
    const pending = st.trusted !== keychain;
    $("#tm-pending").hidden = !pending;
    $("#tm-pending").textContent = pending ? t("tm_pending", "Değişiklik KeyLab'ı yeniden başlatınca geçerli olur.") : "";
  }

  async function refresh() {
    try { const r = await fetch("/api/trusted-mac", { headers: hdr }); st = r.ok ? await r.json() : { portable: false }; } catch (e) { st = { portable: false }; }
    paint(); return st;
  }

  $("#tm-toggle").addEventListener("click", async () => {
    const want = !st.trusted, b = $("#tm-toggle"); b.disabled = true;
    try {
      const r = await fetch("/api/trusted-mac", { method: "POST", headers: hdr, body: JSON.stringify({ enabled: want }) });
      if (!r.ok) { toast((await r.text()).trim() || t("tm_err", "Ayar kaydedilemedi.")); return; }
      st = await r.json(); paint();
    } catch (e) { toast(t("tm_err", "Ayar kaydedilemedi.")); } finally { b.disabled = false; }
  });

  return refresh();
}
