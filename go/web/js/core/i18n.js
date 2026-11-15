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
      quit_ok: "Çıkış yap", quit_ok_force: "Yine de çık", quit_done_title: "KeyLab kapatıldı", quit_done_plain: "Temizlik tamam. Kapat'a basınca KeyLab durur.",
      quit_done_usb: "USB çıkarıldı", quit_done_deferred: "USB uygulama kapandıktan hemen sonra çıkarılacak",
      quit_cleaned_h: "Temizlendi", quit_remains_h: "Bu bilgisayarda kalanlar", quit_c_terms: "Açık terminaller kapatıldı",
      quit_c_agent: "Özel ssh-agent sonlandırıldı (anahtarlar bellekten silindi)", quit_c_tmp: "Geçici klasör silindi (Python ortamı, tarayıcı profili)",
      quit_r_macos: "macOS'un kendi izleri (Spotlight, son kullanılanlar, sistem günlükleri)", quit_r_keychain: "Keychain kayıtları (passphrase, knock dizisi)",
      quit_r_agent: "Kendi ssh-agent'ınıza eklenen anahtarlar", quit_done_nousb: "USB çıkarılamadı, elle çıkarın",
      srv_sshadd_guest: "Eylemlerin parola sormadan bağlanması için anahtarı bu oturumun özel ajanına yükler. Passphrase terminalde sorulur; şifreli kasaya eklediyseniz açılışta kendiliğinden yüklenir.",
      srv_vault_title: "Passphrase (şifreli kasa)", srv_vault_in: "kasada kayıtlı", srv_vault_go: "Kasaya git",
      srv_vault_desc: "Misafir (USB) modda Keychain kullanılmaz. Passphrase'i Ayarlar > Şifreli passphrase kasası'ndan ekleyin: açılışta ana parolayı yazınca anahtar kendiliğinden ajana yüklenir. Keychain'i kendi güvendiğiniz Mac'inizde Ayarlar > Bu Mac'te Keychain ile açabilirsiniz.",
      srv_knock_guest: "Misafir (USB) modda knock dizisi saklanamaz (Keychain yok): knock gerektiren sunucuya bağlanmak için Ayarlar > Bu Mac'te Keychain ile güvendiğiniz Mac'te Keychain'i açın.",
      srv_vault_wiz: "Misafir (USB) modda Keychain kullanılmaz: passphrase'i Ayarlar > Şifreli passphrase kasası'ndan ekleyin (parolalı anahtar kullanıyorsanız).",
      srv_sshadd_wiz_guest: "Eylemlerin parola sormadan bağlanması için (bu oturumun özel ajanı; passphrase terminalde sorulur)",
      nav_monitor: "Kaynaklar", mon_disk: "Disk", mon_procs_h: "Şu an çalışanlar", mon_th_name: "Süreç", mon_th_kind: "Tür", mon_th_time: "Süre", mon_disk_h: "Diskte kapladığı yer",
      mon_note_h: "Yalnızca KeyLab'ın kendisi ve başlattığı süreçler sayılır (özel ssh-agent, eylem terminalleri, ssh, Python/Ansible). Pencerenin kendisi (WebKit) macOS'un ayrı süreçleridir ve burada görünmez. CPU yüzdesi tek çekirdeğe göredir. Klasör boyutları 30 sn'de bir yenilenir.",
      mon_cpu_sub: "tek çekirdeğe göre · tüm çekirdeklerin {n}%'i ({c} çekirdek)", mon_ram_sub: "bilgisayarın belleğinin {n}%'i ({total})", mon_ram_sub0: "KeyLab ve alt süreçleri",
      mon_disk_sub: "diskte kapladığı yer · birimde boş: {free} / {total}", mon_disk_sub0: "diskte kapladığı yer", mon_up: "KeyLab {d} önce açıldı · {t} terminal, {a} eylem çalışıyor · {g} goroutine", mon_procs_n: "{n} süreç",
      mon_k_self: "KeyLab (ana süreç)", mon_k_agent: "Özel ssh-agent", mon_k_ssh: "ssh", mon_k_python: "Python / Ansible", mon_k_task: "Eylem (keylab run)", mon_k_other: "Diğer",
      mon_d_servers: "Sunucular", mon_d_components: "Paketler", mon_d_state: "Durum ve günlükler", mon_d_cache: "Önbellek (Python ortamı)", mon_temp: "geçici — çıkışta silinir",
      mon_u_d: "g", mon_u_h: "sa", mon_u_m: "dk", mon_u_s: "sn",
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
      tm_card: "Bu Mac'te Keychain", tm_s_on: "güvenilir", tm_s_off: "misafir",
      tm_desc: "USB'den çalışırken varsayılan misafir moddur: bu bilgisayarın Keychain'i ve ssh-agent'ı kullanılmaz, iz kalmaz. Yalnızca KENDİ, güvendiğiniz Mac'inizde bunu açabilirsiniz: passphrase Keychain'de durur ve anahtarlar tek tıkla ajana girer. İşaret USB'ye değil bu bilgisayara yazılır; USB başka bir bilgisayara takılınca misafir mod geçerli kalır. Şifreli kasa yalnız misafir modda vardır.",
      tm_active_keychain: "Şu an: bu Mac'in Keychain'i ve ssh-agent'ı kullanılıyor.", tm_active_guest: "Şu an: misafir mod (özel ssh-agent, Keychain kullanılmıyor).",
      tm_on: "Bu Mac'e güveniyorum: Keychain'i kullan", tm_off: "Bu Mac'e artık güvenme", tm_pending: "Değişiklik KeyLab'ı yeniden başlatınca geçerli olur.", tm_err: "Ayar kaydedilemedi.",
      quit_closing: "KeyLab kapanıyor…", quit_ready: "Kapatabilirsiniz", quit_finish: "Kapat", quit_stopped: "KeyLab durduruldu. Bu pencereyi kapatabilirsiniz.",
      quit_s_terms_run: "Açık terminaller kapatılıyor…", quit_s_terms_done: "{n} terminal oturumu kapatıldı", quit_s_terms_skip: "Açık terminal yoktu",
      quit_s_vault_run: "Şifreli kasa kilitleniyor…", quit_s_vault_done: "Kasa kilitlendi; bellekteki anahtar ve passphrase'ler sıfırlandı",
      quit_s_usb_run: "USB'deki anahtarlar ajandan siliniyor ve USB çıkarılıyor…", quit_s_usb_done: "USB çıkarıldı: {name}",
      quit_s_usb_def: "USB, uygulama kapandıktan hemen sonra çıkarılacak: {name}", quit_s_usb_err: "USB çıkarılamadı, elle çıkarın: {name}",
      quit_s_agent_run: "Özel ssh-agent durduruluyor…", quit_s_agent_done: "Özel ssh-agent durduruldu; anahtarlar bellekten silindi",
      quit_s_tmp_run: "Geçici klasör siliniyor…", quit_s_tmp_done: "Geçici klasör silindi (Python ortamı, tarayıcı profili)",
      quit_s_kc_run: "Keychain denetleniyor…", quit_s_kc_guest: "Bu bilgisayarın Keychain'i ve ssh-agent'ı hiç kullanılmadı", quit_s_kc_kept: "Keychain kayıtlarına dokunulmadı (bu Mac'te kalır)",
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
      quit_ok: "Quit", quit_ok_force: "Quit anyway", quit_done_title: "KeyLab has quit", quit_done_plain: "Cleanup done. KeyLab stops when you press Close.",
      quit_done_usb: "USB ejected", quit_done_deferred: "The USB will be ejected right after the app closes",
      quit_cleaned_h: "Cleaned up", quit_remains_h: "Left on this computer", quit_c_terms: "Open terminals closed",
      quit_c_agent: "Private ssh-agent stopped (keys wiped from memory)", quit_c_tmp: "Temporary folder deleted (Python environment, browser profile)",
      quit_r_macos: "macOS's own traces (Spotlight, recent items, system logs)", quit_r_keychain: "Keychain entries (passphrases, knock sequence)",
      quit_r_agent: "Keys added to your own ssh-agent", quit_done_nousb: "Could not eject the USB, eject it manually",
      srv_sshadd_guest: "Loads the key into this session's private agent so actions can connect without asking. The passphrase is asked in the terminal; if you added it to the encrypted vault it loads by itself at startup.",
      srv_vault_title: "Passphrase (encrypted vault)", srv_vault_in: "stored in the vault", srv_vault_go: "Go to the vault",
      srv_vault_desc: "In guest (USB) mode the Keychain is not used. Add the passphrase under Settings > Encrypted passphrase vault: after you enter the master password at startup the key loads into the agent by itself. On your own trusted Mac you can turn the Keychain on under Settings > Keychain on this Mac.",
      srv_knock_guest: "In guest (USB) mode the knock sequence cannot be stored (no Keychain): to connect to a server that needs knocking, turn the Keychain on under Settings > Keychain on this Mac on a Mac you trust.",
      srv_vault_wiz: "In guest (USB) mode the Keychain is not used: add the passphrase under Settings > Encrypted passphrase vault (if your key has a passphrase).",
      srv_sshadd_wiz_guest: "So actions can connect without asking (this session's private agent; the passphrase is asked in the terminal)",
      nav_monitor: "Resources", mon_disk: "Disk", mon_procs_h: "Running now", mon_th_name: "Process", mon_th_kind: "Kind", mon_th_time: "Uptime", mon_disk_h: "Disk space used",
      mon_note_h: "Only KeyLab itself and the processes it started are counted (private ssh-agent, action terminals, ssh, Python/Ansible). The window itself (WebKit) consists of separate macOS processes and is not shown here. CPU is relative to one core. Folder sizes refresh every 30 s.",
      mon_cpu_sub: "relative to one core · {n}% of all cores ({c} cores)", mon_ram_sub: "{n}% of this computer's memory ({total})", mon_ram_sub0: "KeyLab and its child processes",
      mon_disk_sub: "disk space used · free on the volume: {free} / {total}", mon_disk_sub0: "disk space used", mon_up: "KeyLab started {d} ago · {t} terminal(s), {a} action(s) running · {g} goroutines", mon_procs_n: "{n} processes",
      mon_k_self: "KeyLab (main process)", mon_k_agent: "Private ssh-agent", mon_k_ssh: "ssh", mon_k_python: "Python / Ansible", mon_k_task: "Action (keylab run)", mon_k_other: "Other",
      mon_d_servers: "Servers", mon_d_components: "Packages", mon_d_state: "State and logs", mon_d_cache: "Cache (Python environment)", mon_temp: "temporary — deleted on exit",
      mon_u_d: "d", mon_u_h: "h", mon_u_m: "m", mon_u_s: "s",
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
      tm_card: "Keychain on this Mac", tm_s_on: "trusted", tm_s_off: "guest",
      tm_desc: "When running from a USB stick, KeyLab defaults to guest mode: this computer's Keychain and ssh-agent are not used and no traces are left. Only on YOUR OWN trusted Mac can you turn this on: passphrases stay in the Keychain and keys load into the agent in one go. The marker is written to this computer, not the USB stick; if the stick is plugged into another computer, guest mode stays in effect. The encrypted vault exists only in guest mode.",
      tm_active_keychain: "Currently: this Mac's Keychain and ssh-agent are in use.", tm_active_guest: "Currently: guest mode (private ssh-agent, no Keychain).",
      tm_on: "I trust this Mac: use the Keychain", tm_off: "Stop trusting this Mac", tm_pending: "The change takes effect after restarting KeyLab.", tm_err: "Could not save the setting.",
      quit_closing: "KeyLab is closing…", quit_ready: "You can close now", quit_finish: "Close", quit_stopped: "KeyLab has stopped. You can close this window.",
      quit_s_terms_run: "Closing open terminals…", quit_s_terms_done: "{n} terminal session(s) closed", quit_s_terms_skip: "No open terminals",
      quit_s_vault_run: "Locking the encrypted vault…", quit_s_vault_done: "Vault locked; the in-memory key and passphrases were wiped",
      quit_s_usb_run: "Removing the USB keys from the agent and ejecting the USB stick…", quit_s_usb_done: "USB ejected: {name}",
      quit_s_usb_def: "The USB stick will be ejected right after the app closes: {name}", quit_s_usb_err: "Could not eject the USB stick, eject it manually: {name}",
      quit_s_agent_run: "Stopping the private ssh-agent…", quit_s_agent_done: "Private ssh-agent stopped; keys wiped from memory",
      quit_s_tmp_run: "Deleting the temporary folder…", quit_s_tmp_done: "Temporary folder deleted (Python environment, browser profile)",
      quit_s_kc_run: "Checking the Keychain…", quit_s_kc_guest: "This computer's Keychain and ssh-agent were never used", quit_s_kc_kept: "Keychain entries were left untouched (they stay on this Mac)",
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
