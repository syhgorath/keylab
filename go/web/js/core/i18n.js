/* KeyLab çeviri motoru (TR/EN). pocketui'deki js/core/i18n.js'ten esinlenildi; farklar: metinler textContent ile yazılır
   (HTML enjeksiyonu olmaz) ve saklama anahtarı "keylab.lang". Eksik anahtar Türkçeye, o da yoksa t() çağrısındaki varsayılana düşer;
   böylece çeviri parça parça eklenebilir. Yeni arayüz metinleri: HTML'de data-i18n="anahtar" (metin), data-i18n-placeholder,
   data-i18n-title, data-i18n-aria; JS'te t("anahtar", "Türkçe varsayılan"). */
(function () {
  "use strict";

  const STORAGE_KEY = "keylab.lang";
  const SUPPORTED_LANGS = ["tr", "en"];
  let currentLang = "tr";

  const DICTS = {
    tr: {
      nav_general: "Genel", nav_security: "Güvenlik", nav_system: "Sistem",
      nav_dash: "Pano", nav_servers: "Sunucular", nav_packages: "Paketler",
      nav_lynis: "Lynis denetimi", nav_logs: "Günlükler", nav_settings: "Ayarlar",
      nav_compare: "Karşılaştır",
      nav_reports: "Raporlar",
      search_ph: "Sunucu, paket ara…",
      tree_title: "Sunucular", tree_add: "+ Ekle", tree_search: "Sunucu veya IP ara…", tree_none: "Eşleşen sunucu yok.",
      term_find_none: "Bulunamadı", term_quick_sudo: "sudo parolası sorabilir", term_ip_copied: "IP panoya kopyalandı: ",
      ff_all: "Tümü", ff_up: "Çevrimiçi", ff_down: "Kapalı", ff_warn: "Uyarılı", ff_nokey: "Anahtarsız",
      lang_title: "Dil / Language",
      st_invalid: "yapılandırma geçersiz", st_checking: "denetleniyor", st_up: "çevrimiçi", st_knock: "🔒 kapalı — knock gerekir",
      st_closed: "port kapalı (makine yanıt veriyor)", st_down: "bağlantı yok",
      theme_light: "Açık",
      imp_selected: "seçili", imp_loading: "Taranıyor…", imp_fail: "Hata:", imp_done: "sunucu eklendi", imp_skipped: "zaten vardı (atlandı)", imp_errors: "hata",
      term_sub: "Gömülü terminal: kayıtlı sunucuya ssh ile bağlanır. Parola ve passphrase burada, terminalde sorulur.",
      term_foot: "Küçültmek (sarı) oturumu kapatmaz; kırmızı düğme bu sunucunun tüm terminallerini, sekmedeki ✕ yalnız o oturumu sonlandırır. Terminal açma/kapama durum klasöründe terminal-audit.log dosyasına yazılır.",
      term_max: "En fazla 4 terminal oturumu açık olabilir; birini kapatın.",
      quit_btn: "Çıkış", quit_btn_title: "Çıkış (terminalleri kapatır; USB'den çalışıyorsa USB'yi çıkarır)", quit_title: "Çıkış", quit_sub_force: "Çalışan iş var",
      quit_what: "Açık terminaller kapanır, KeyLab durur.", quit_running: "iş çalışıyor", quit_running2: "çıkarsanız yarıda kesilir.",
      quit_usb: "USB çıkarılır", quit_usb2: "bu USB'deki anahtarlar ssh-agent'tan silinir.", quit_keychain: "Keychain kayıtlarına dokunulmaz.",
      quit_ok: "Çıkış yap", quit_ok_force: "Yine de çık", quit_done_title: "KeyLab kapatıldı", quit_done_plain: "KeyLab durduruldu. Bu pencereyi kapatabilirsiniz.",
      quit_done_usb: "USB çıkarıldı", quit_done_deferred: "USB uygulama kapandıktan hemen sonra çıkarılacak",
      quit_cleaned_h: "Temizlendi", quit_remains_h: "Bu bilgisayarda kalanlar", quit_c_terms: "Açık terminaller kapatıldı",
      quit_c_agent: "Özel ssh-agent sonlandırıldı (anahtarlar bellekten silindi)", quit_c_tmp: "Geçici klasör silindi (Python ortamı, tarayıcı profili)",
      quit_r_macos: "macOS'un kendi izleri (Spotlight, son kullanılanlar, sistem günlükleri)", quit_r_keychain: "Keychain kayıtları (passphrase, knock dizisi)",
      quit_r_agent: "Kendi ssh-agent'ınıza eklenen anahtarlar", quit_done_nousb: "USB çıkarılamadı, elle çıkarın",
      vault_lock_h: "KeyLab kilitli", vault_lock_sub: "Devam etmek için ana parolayı yazın. Anahtar passphrase'leri bu parolayla şifreli kasada durur.",
      vault_pw: "Ana parola", vault_pw2: "Ana parola (tekrar)", vault_unlock: "Kilidi aç", vault_wrong: "Ana parola yanlış.", vault_err: "Kasa açılamadı.",
      vault_wait: "Çok fazla yanlış deneme; {n} sn bekleyin.", vault_opened: "Kasa açıldı: {n} anahtar ajana yüklendi.", vault_failed: "Yüklenemeyen: {names}",
      vault_relocked: "Kasa kilitlendi (boşta kalma ya da elle). Devam etmek için ana parolayı yazın.", vault_lock_title_btn: "Kasayı kilitle (ana parola olmadan açılmaz)",
      vault_card: "Şifreli passphrase kasası", vault_s_none: "kasa yok", vault_s_open: "açık", vault_s_locked: "kilitli",
      vault_card_desc: "SSH anahtar passphrase'leri bu USB'de, ana parolayla şifreli bir kasada durur (AES-256-GCM). Açılışta ana parolayı yazarsınız; anahtarlar yalnızca bu oturumun özel ajanına yüklenir, eylemler parola sormaz. Ana parolayı unutursanız kasa açılamaz (kurtarma yolu yok).",
      vault_create: "Kasa oluştur", vault_mismatch: "Parolalar aynı değil.", vault_short: "Ana parola en az 8 karakter olmalı.", vault_created: "Kasa oluşturuldu ve açık. Anahtar passphrase'lerini aşağıdan ekleyin.",
      vault_stored_h: "Kasadaki passphrase'ler", vault_none_stored: "Henüz passphrase eklenmedi.", vault_rm: "Sil", vault_srv: "Sunucu", vault_pp_ph: "Anahtar passphrase'i", vault_add: "Kasaya ekle",
      vault_add_hint: "Eklenen passphrase önce anahtarı gerçekten açıyor mu diye denenir; yanlışsa kaydedilmez.", vault_need: "Sunucu ve passphrase gerekli.",
      vault_added: "{s}: passphrase kasaya eklendi ve anahtar ajana yüklendi.", vault_removed: "{s}: passphrase kasadan silindi.", vault_lock_now: "Şimdi kilitle",
      vault_idle_n: "{n} dk işlem yapılmazsa kilitlenir", vault_idle_off: "otomatik kilit kapalı", vault_chg: "Ana parolayı değiştir", vault_old: "Mevcut ana parola",
      vault_new: "Yeni ana parola", vault_chg_go: "Parolayı değiştir", vault_changed: "Ana parola değiştirildi.",
      term_end: "Oturumu kapat", term_exited: "oturum bitti", term_task_done: "eylem bitti", term_task_arm: "Eylem hâlâ çalışıyor: kapatmak onu yarıda keser ve sunucuyu yarım bırakabilir. Yine de kapatmak için bir kez daha basın.", term_lib_missing: "xterm.js yüklenemedi.",
      term_input_fail: "Girdi iletilemedi: oturum kapanmış olabilir.", term_conn_lost: "Akış koptu; oturum kapanmış olabilir.",
    },
    en: {
      nav_general: "General", nav_security: "Security", nav_system: "System",
      nav_dash: "Dashboard", nav_servers: "Servers", nav_packages: "Packages",
      nav_lynis: "Lynis audit", nav_logs: "Logs", nav_settings: "Settings",
      nav_compare: "Compare",
      nav_reports: "Reports",
      search_ph: "Search servers, packages…",
      tree_title: "Servers", tree_add: "+ Add", tree_search: "Search server or IP…", tree_none: "No matching server.",
      term_find_none: "Not found", term_quick_sudo: "may ask for the sudo password", term_ip_copied: "IP copied to clipboard: ",
      ff_all: "All", ff_up: "Online", ff_down: "Offline", ff_warn: "With warnings", ff_nokey: "No key",
      lang_title: "Dil / Language",
      st_invalid: "invalid configuration", st_checking: "checking", st_up: "online", st_knock: "🔒 closed — knock required",
      st_closed: "port closed (host is responding)", st_down: "no connection",
      theme_light: "Light",
      imp_selected: "selected", imp_loading: "Scanning…", imp_fail: "Error:", imp_done: "server(s) added", imp_skipped: "already existed (skipped)", imp_errors: "error(s)",
      term_sub: "Embedded terminal: connects to the registered server over ssh. Passwords and passphrases are asked here, in the terminal.",
      term_foot: "Minimizing (yellow) does not close the session; the red button terminates all terminals of this server, the ✕ on a tab only that session. Opening/closing a terminal is logged to terminal-audit.log in the state folder.",
      term_max: "At most 4 terminal sessions can be open; close one.",
      quit_btn: "Quit", quit_btn_title: "Quit (closes terminals; ejects the USB if you run from one)", quit_title: "Quit", quit_sub_force: "A job is running",
      quit_what: "Open terminals are closed and KeyLab stops.", quit_running: "job(s) running", quit_running2: "quitting interrupts them.",
      quit_usb: "The USB is ejected", quit_usb2: "the keys on this USB are removed from ssh-agent.", quit_keychain: "Keychain entries are not touched.",
      quit_ok: "Quit", quit_ok_force: "Quit anyway", quit_done_title: "KeyLab has quit", quit_done_plain: "KeyLab stopped. You can close this window.",
      quit_done_usb: "USB ejected", quit_done_deferred: "The USB will be ejected right after the app closes",
      quit_cleaned_h: "Cleaned up", quit_remains_h: "Left on this computer", quit_c_terms: "Open terminals closed",
      quit_c_agent: "Private ssh-agent stopped (keys wiped from memory)", quit_c_tmp: "Temporary folder deleted (Python environment, browser profile)",
      quit_r_macos: "macOS's own traces (Spotlight, recent items, system logs)", quit_r_keychain: "Keychain entries (passphrases, knock sequence)",
      quit_r_agent: "Keys added to your own ssh-agent", quit_done_nousb: "Could not eject the USB, eject it manually",
      vault_lock_h: "KeyLab is locked", vault_lock_sub: "Enter the master password to continue. Key passphrases are kept in a vault encrypted with it.",
      vault_pw: "Master password", vault_pw2: "Master password (again)", vault_unlock: "Unlock", vault_wrong: "Wrong master password.", vault_err: "Could not unlock the vault.",
      vault_wait: "Too many wrong attempts; wait {n} s.", vault_opened: "Vault unlocked: {n} key(s) loaded into the agent.", vault_failed: "Could not load: {names}",
      vault_relocked: "The vault was locked (idle or manual). Enter the master password to continue.", vault_lock_title_btn: "Lock the vault (cannot be opened without the master password)",
      vault_card: "Encrypted passphrase vault", vault_s_none: "no vault", vault_s_open: "unlocked", vault_s_locked: "locked",
      vault_card_desc: "SSH key passphrases are kept on this USB in a vault encrypted with a master password (AES-256-GCM). You type the master password at startup; keys are loaded only into this session's private agent and actions do not ask for passphrases. If you forget the master password the vault cannot be opened (no recovery).",
      vault_create: "Create vault", vault_mismatch: "Passwords do not match.", vault_short: "The master password must be at least 8 characters.", vault_created: "Vault created and unlocked. Add key passphrases below.",
      vault_stored_h: "Passphrases in the vault", vault_none_stored: "No passphrases added yet.", vault_rm: "Remove", vault_srv: "Server", vault_pp_ph: "Key passphrase", vault_add: "Add to vault",
      vault_add_hint: "A passphrase is first tested against the key; it is not saved if it is wrong.", vault_need: "Server and passphrase are required.",
      vault_added: "{s}: passphrase added to the vault and key loaded into the agent.", vault_removed: "{s}: passphrase removed from the vault.", vault_lock_now: "Lock now",
      vault_idle_n: "locks after {n} min of inactivity", vault_idle_off: "auto-lock is off", vault_chg: "Change master password", vault_old: "Current master password",
      vault_new: "New master password", vault_chg_go: "Change password", vault_changed: "Master password changed.",
      term_end: "End session", term_exited: "session ended", term_task_done: "action finished", term_task_arm: "The action is still running: closing it interrupts it and may leave the server half-configured. Press once more to close anyway.", term_lib_missing: "xterm.js could not be loaded.",
      term_input_fail: "Input could not be sent: the session may have closed.", term_conn_lost: "Stream lost; the session may have closed.",
    },
  };

  const getLang = () => currentLang;

  function t(key, fallback) {
    const d = DICTS[currentLang];
    if (d && d[key] != null) return d[key];
    if (DICTS.tr[key] != null) return DICTS.tr[key];
    return fallback != null ? fallback : key;
  }

  let applyI18n = function (root) {
    const base = root || document;
    const each = (attr, fn) => base.querySelectorAll("[" + attr + "]").forEach(el => fn(el, t(el.getAttribute(attr), null)));
    each("data-i18n", (el, v) => { if (v != null) el.textContent = v; });
    each("data-i18n-title", (el, v) => { if (v != null) el.title = v; });
    each("data-i18n-placeholder", (el, v) => { if (v != null) el.placeholder = v; });
    each("data-i18n-aria", (el, v) => { if (v != null) el.setAttribute("aria-label", v); });
    const badge = document.getElementById("lang-cur");
    if (badge) badge.textContent = currentLang.toUpperCase();
  };

  function setLang(lang) {
    if (!SUPPORTED_LANGS.includes(lang)) lang = "tr";
    currentLang = lang;
    try { localStorage.setItem(STORAGE_KEY, lang); } catch (_) { /* depolama kapalı olabilir */ }
    document.documentElement.lang = lang;
    applyI18n(document);
    window.dispatchEvent(new CustomEvent("keylab:langchange", { detail: { lang } }));
  }

  function init() {
    let saved = null;
    try { saved = localStorage.getItem(STORAGE_KEY); } catch (_) { /* yoksay */ }
    currentLang = SUPPORTED_LANGS.includes(saved) ? saved : "tr";
    document.documentElement.lang = currentLang;
    applyI18n(document);
  }

  /* ---- Ekranda çizilen metin için ifade çevirisi (geçiş katmanı) ----
     Çizim kodları Türkçe metni doğrudan üretir. İngilizce seçiliyken burada, tam eşleşen metin düğümleri (ve title / placeholder /
     aria-label öznitelikleri) js/core/i18n-phrases-en.js tablosuna göre değiştirilir; Türkçeye dönünce özgün metin geri konur.
     Tam eşleşme aranır: kullanıcı verisi (sunucu adı, günlük satırı) bir ifadeyle tıpatıp aynı değilse asla değişmez.
     Eşleşmeyen metin Türkçe kalır (çeviri parça parça eklenir). Kalıcı çözüm: metinleri data-i18n / t() anahtarlarına taşımak. */
  const PHRASES = window.KeyLabPhrasesEN || { exact: {}, patterns: [] };
  const tracked = new Map(); // düğüm -> özgün değer (metin düğümü için nodeValue; öznitelik için {attr: değer})
  const ATTRS = ["title", "placeholder", "aria-label"];
  const SKIP = new Set(["SCRIPT", "STYLE", "TEXTAREA", "INPUT", "PRE", "CODE"]);
  function trPhrase(raw) {
    const k = raw.replace(/\s+/g, " ").trim();
    if (!k) return null;
    if (PHRASES.exact[k] != null) return PHRASES.exact[k];
    for (const [re, rep] of PHRASES.patterns) if (re.test(k)) return k.replace(re, rep);
    return null;
  }
  let busy = false, scheduled = false;
  function translateTree() {
    if (busy) return;
    busy = true;
    try {
      if (currentLang !== "en") {
        tracked.forEach((orig, node) => {
          if (!node.isConnected) { tracked.delete(node); return; }
          if (node.nodeType === 3) node.nodeValue = orig;
          else Object.keys(orig).forEach(a => node.setAttribute(a, orig[a]));
        });
        tracked.clear();
        return;
      }
      const w = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT, {
        acceptNode: n => (n.parentNode && SKIP.has(n.parentNode.nodeName)) ? NodeFilter.FILTER_REJECT : NodeFilter.FILTER_ACCEPT,
      });
      let n;
      while ((n = w.nextNode())) {
        if (tracked.has(n)) continue;
        const r = trPhrase(n.nodeValue);
        if (r == null) continue;
        const lead = n.nodeValue.match(/^\s*/)[0], trail = n.nodeValue.match(/\s*$/)[0];
        tracked.set(n, n.nodeValue);
        n.nodeValue = lead + r + trail;
      }
      document.body.querySelectorAll("[title],[placeholder],[aria-label]").forEach(el => {
        const orig = tracked.get(el) || {};
        let changed = false;
        ATTRS.forEach(a => {
          const v = el.getAttribute(a);
          if (v == null) return;
          const r = trPhrase(orig[a] != null ? orig[a] : v);
          if (r != null && r !== v) { orig[a] = orig[a] != null ? orig[a] : v; el.setAttribute(a, r); changed = true; }
        });
        if (changed) tracked.set(el, orig);
      });
    } finally { busy = false; }
  }
  function scheduleTranslate() {
    if (scheduled) return;
    scheduled = true;
    // setTimeout: arka plandaki sekmede requestAnimationFrame durur; kısa bir gecikme art arda çizimleri de tek turda toplar
    setTimeout(() => { scheduled = false; translateTree(); }, 20);
  }
  const origApply = applyI18n;
  applyI18n = function (root) { origApply(root); translateTree(); };

  window.KeyLabI18n = { getLang, setLang, t, applyI18n, translateTree, SUPPORTED_LANGS, DICTS };
  init();
  new MutationObserver(() => { if (!busy) scheduleTranslate(); }).observe(document.documentElement, { childList: true, subtree: true, characterData: true });
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", () => applyI18n(document));
})();
