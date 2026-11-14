/* Gömülü terminal (xterm.js): kayıtlı bir sunucuya ssh -tt, sunucu tarafı PTY (bkz. ../../term.go).
   ÇOKLU OTURUM: en çok 4 oturum (sunucu sınırıyla aynı). Pencereyi KÜÇÜLTMEK oturumu kapatmaz; oturumlar arkada çalışır ve altta
   "terminal çubuğunda" görünür. Oturumu KAPATMAK (✕) ssh'ı sonlandırır. Sayfa yenilenirse çubuk /api/term/list ile yeniden kurulur;
   bir oturum açılınca sunucu son çıktıyı (256 KB) yeniden gönderir. 30 dk girdi olmayan oturumu sunucu kendiliğinden kapatır.
   Güvenlik: sunucudan gelen metin terminale YALNIZ xterm'in kendi ayrıştırıcısıyla yazılır; hata iletileri textContent ile gösterilir,
   terminale elle metin yazılırken yalnız sabit dizgeler ve sayılar kullanılır (kontrol karakterleri sızmaz). Tüm arayüz metni DOM API'siyle
   (textContent) kurulur; sunucu adı HTML olarak yorumlanmaz. */
(function () {
  "use strict";
  const H = { "X-Requested-With": "sshpocket", "Content-Type": "application/json" };
  const post = (path, body) => fetch(path, { method: "POST", headers: H, body: JSON.stringify(body) });
  const b64ToBytes = s => Uint8Array.from(atob(s), c => c.charCodeAt(0));
  const tr = (k, d) => (window.KeyLabI18n ? window.KeyLabI18n.t(k, d) : d);
  const MAX = 4;

  // s: { id, server, term, fit, es, ro, host, closed, queue, sending, unread, built, note }
  let sessions = [], active = null;

  const $ = s => document.querySelector(s);
  const modal = () => $("#termmodal");
  const modalOpen = () => modal().classList.contains("open");
  function setNote(msg) { const n = $("#term-note"); n.textContent = msg || ""; n.hidden = !msg; }

  async function flushInput(s) {
    if (s.sending || !s.queue) return;
    s.sending = true;
    try {
      while (s.queue && !s.closed) {
        const data = s.queue; s.queue = "";
        const r = await post("/api/term/input", { id: s.id, data });
        if (!r.ok) { if (s === active) setNote(tr("term_input_fail", "Girdi iletilemedi: oturum kapanmış olabilir.")); break; }
      }
    } catch { if (s === active) setNote(tr("term_input_fail", "Girdi iletilemedi: oturum kapanmış olabilir.")); }
    s.sending = false;
  }

  const label = s => { const same = sessions.filter(x => x.server === s.server); return same.length > 1 ? `${s.server} #${same.indexOf(s) + 1}` : s.server; };

  function chip(s, kind) {
    const el = document.createElement("div");
    el.className = "term-chip" + (s === active && modalOpen() ? " active" : "") + (s.unread ? " unread" : "");
    el.setAttribute("role", kind === "tab" ? "tab" : "button"); el.tabIndex = 0;
    el.setAttribute("aria-selected", String(s === active));
    const dot = document.createElement("span"); dot.className = "status-dot " + (s.closed ? "bad" : "on"); dot.setAttribute("aria-hidden", "true");
    const name = document.createElement("span"); name.className = "mono"; name.textContent = ">_ " + label(s);
    const x = document.createElement("button"); x.type = "button"; x.className = "term-chip-x"; x.textContent = "✕";
    x.title = tr("term_end", "Oturumu kapat"); x.setAttribute("aria-label", tr("term_end", "Oturumu kapat") + ": " + label(s));
    x.addEventListener("click", e => { e.stopPropagation(); closeSession(s); });
    el.append(dot, name, x);
    const go = () => show(s);
    el.addEventListener("click", go);
    el.addEventListener("keydown", e => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); go(); } });
    return el;
  }

  // pencere başlığındaki durum: bağlanıyor… / bağlı / oturum bitti / bağlanamadı (renkli nokta + metin)
  function renderState() {
    const el = $("#term-state"); if (!el) return;
    const s = active;
    const [cls, text] = !s ? ["", ""] : s.closed ? (s.note ? ["bad", tr("term_state_failed", "bağlanamadı")] : ["bad", tr("term_state_ended", "oturum bitti")]) : !s.id ? ["warn", tr("term_state_connecting", "bağlanıyor…")] : ["ok", tr("term_state_connected", "bağlı")];
    el.className = "term-window-state" + (cls ? " " + cls : ""); el.textContent = text;
  }

  function toggleMax() {
    const w = $("#term-window"), on = !w.classList.contains("maximized");
    w.classList.toggle("maximized", on);
    const b = $("[data-term-max]"); if (b) { b.setAttribute("aria-pressed", String(on)); b.title = on ? tr("term_restore", "Eski boyuta dön") : tr("term_maximize", "Tam ekran / eski boyut"); }
    if (active && active.fit) { try { active.fit.fit(); } catch {} } // ResizeObserver de yeniden boyutlar; hemen sığdır
  }

  function renderBars() {
    renderState();
    const tabs = $("#term-tabs"), dock = $("#term-dock");
    tabs.replaceChildren(...sessions.map(s => chip(s, "tab")));
    dock.replaceChildren(...sessions.map(s => chip(s, "dock")));
    dock.hidden = !sessions.length;
  }

  function buildView(s) {
    if (s.built) return;
    s.built = true;
    s.host = document.createElement("div"); s.host.className = "term-pane"; s.host.hidden = true;
    $("#term-host").appendChild(s.host);
    s.term = new Terminal({ cursorBlink: true, fontFamily: '"MesloLGS NF", "FiraCode Nerd Font", "JetBrains Mono Nerd Font", ui-monospace, Menlo, Consolas, monospace', fontSize: 13, scrollback: 5000,
      theme: { background: "#0e1116", foreground: "#d6dbe4" }, allowProposedApi: false });
    s.fit = new FitAddon.FitAddon();
    s.term.loadAddon(s.fit);
    // arama (Cmd+F, Windows/Linux'ta Ctrl+Shift+F): Ctrl+F kabukta "bir karakter ileri" olduğu için ele alınmaz
    if (typeof SearchAddon !== "undefined") { s.search = new SearchAddon.SearchAddon(); s.term.loadAddon(s.search); }
    s.term.attachCustomKeyEventHandler(e => {
      if (s.search && (e.metaKey || (e.ctrlKey && e.shiftKey)) && (e.key === "f" || e.key === "F")) { if (e.type === "keydown") { e.preventDefault(); openSearch(); } return false; }
      return true;
    });
    // bağlantılar: YALNIZ Cmd/Ctrl+tıklamayla açılır (sunucu çıktısındaki bağlantıya yanlışlıkla tıklanmasın); yalnız http(s), noopener.
    if (typeof WebLinksAddon !== "undefined") {
      s.term.loadAddon(new WebLinksAddon.WebLinksAddon((ev, uri) => { if ((ev.metaKey || ev.ctrlKey) && /^https?:\/\//i.test(uri)) window.open(uri, "_blank", "noopener,noreferrer"); }));
    }
    // IPv4 adresleri: Cmd/Ctrl+tıklamayla panoya kopyalanır (tarayıcıda açılacak bir şey yok; ssh/ping için yapıştırılır)
    s.term.registerLinkProvider({
      provideLinks(y, cb) {
        const line = s.term.buffer.active.getLine(y - 1); if (!line) return cb(undefined);
        const text = line.translateToString(true), out = [], re = /(?<![\d.])(?:(?:25[0-5]|2[0-4]\d|1?\d?\d)\.){3}(?:25[0-5]|2[0-4]\d|1?\d?\d)(?![\d.])/g;
        for (let m; (m = re.exec(text));) {
          const ip = m[0];
          out.push({ text: ip, range: { start: { x: m.index + 1, y }, end: { x: m.index + ip.length, y } },
            activate(ev) { if (!(ev.metaKey || ev.ctrlKey)) return; try { navigator.clipboard.writeText(ip).then(() => toast(tr("term_ip_copied", "IP panoya kopyalandı: ") + ip)); } catch { /* pano yok */ } } });
        }
        cb(out.length ? out : undefined);
      },
    });
  }

  /* ---- terminal içi arama ---- */
  const toast = m => window.dispatchEvent(new CustomEvent("keylab:toast", { detail: m }));
  const findOpts = () => ({ caseSensitive: $("#term-find-case").checked });
  function openSearch() {
    if (!active || !active.search) return;
    const bar = $("#term-search"), inp = $("#term-find");
    bar.hidden = false;
    const sel = active.term.getSelection();
    if (sel && sel.length < 80 && !/\n/.test(sel)) inp.value = sel;
    inp.focus(); inp.select();
    if (inp.value) doFind(true);
  }
  function closeSearch() {
    const bar = $("#term-search"); if (bar.hidden) return;
    bar.hidden = true; $("#term-find-status").textContent = "";
    if (active && active.term) { try { active.term.clearSelection(); } catch {} active.term.focus(); }
  }
  function doFind(forward, again) {
    if (!active || !active.search) return;
    const q = $("#term-find").value, st = $("#term-find-status");
    if (!q) { st.textContent = ""; try { active.term.clearSelection(); } catch {} return; }
    const ok = forward ? active.search.findNext(q, { ...findOpts(), incremental: !again }) : active.search.findPrevious(q, findOpts());
    st.textContent = ok ? "" : tr("term_find_none", "Bulunamadı");
  }
  /* ---- hızlı komutlar: SABİT, salt okunur liste (koda gömülü; sunucudan/yapılandırmadan/DOM'dan gelmez). Tıklayınca komut Enter ile
     birlikte AÇIK oturumun girdisine gider. UYARI: komut o an terminalde çalışan programa yazılır (editör, parola istemi vb. olabilir);
     kabuk isteminde olduğunuzdan emin olun. sudo'lu olanlar parolayı terminalde sorar. */
  const QUICK = [
    ["uptime", "uptime"], ["df -h", "df -h"], ["ip addr", "ip addr"], ["ufw", "sudo ufw status verbose"],
    ["ssh", "systemctl status ssh"], ["fail2ban", "sudo fail2ban-client status"], ["timedatectl", "timedatectl"],
  ];
  function sendQuick(cmd) {
    if (!active || active.closed || !active.id) return;
    active.queue += cmd + "\n"; flushInput(active); active.term.focus();
  }
  function initQuick() {
    const box = $("#term-quick");
    QUICK.forEach(([label, cmd]) => {
      const b = document.createElement("button"); b.type = "button"; b.className = "term-quick-chip"; b.textContent = label;
      b.title = cmd + (cmd.startsWith("sudo ") ? " (" + tr("term_quick_sudo", "sudo parolası sorabilir") + ")" : "");
      b.addEventListener("click", () => sendQuick(cmd));
      box.appendChild(b);
    });
  }

  function initSearchUi() {
    $("#term-find").addEventListener("input", () => doFind(true));
    $("#term-find").addEventListener("keydown", e => {
      if (e.key === "Enter") { e.preventDefault(); doFind(!e.shiftKey, true); }
      else if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); closeSearch(); }
    });
    $("#term-find-next").addEventListener("click", () => doFind(true, true));
    $("#term-find-prev").addEventListener("click", () => doFind(false, true));
    $("#term-find-case").addEventListener("change", () => doFind(true));
    $("#term-find-close").addEventListener("click", closeSearch);
  }

  function connect(s) { // oturum kimliği biliniyor: girdi + akış
    s.term.onData(d => { s.queue += d; flushInput(s); });
    let rt; s.term.onResize(({ rows, cols }) => { clearTimeout(rt); rt = setTimeout(() => { if (!s.closed) post("/api/term/resize", { id: s.id, rows, cols }).catch(() => {}); }, 120); });
    s.ro = new ResizeObserver(() => { if (!s.host.hidden) { try { s.fit.fit(); } catch {} } }); s.ro.observe(s.host);
    const es = s.es = new EventSource("/api/term/stream?id=" + encodeURIComponent(s.id));
    es.addEventListener("data", ev => {
      try { s.term.write(b64ToBytes(ev.data)); } catch {}
      if (s !== active || !modalOpen()) { if (!s.unread) { s.unread = true; renderBars(); } }
    });
    es.addEventListener("exit", ev => {
      s.closed = true; es.close();
      const code = String(parseInt(ev.data, 10) || 0); // yalnız sayı
      s.term.write("\r\n\x1b[90m[" + tr("term_exited", "oturum bitti") + ": " + code + "]\x1b[0m\r\n");
      renderBars();
    });
    es.onerror = () => { if (!s.closed && s === active) setNote(tr("term_conn_lost", "Akış koptu; oturum kapanmış olabilir.")); };
  }

  function show(s) {
    if (typeof Terminal === "undefined" || typeof FitAddon === "undefined") { alert(tr("term_lib_missing", "xterm.js yüklenemedi.")); return; }
    active = s; s.unread = false;
    modal().classList.add("open");
    sessions.forEach(x => { if (x.host) x.host.hidden = x !== s; });
    buildView(s);
    s.host.hidden = false;
    if (!s.opened) { s.term.open(s.host); s.opened = true; }
    closeSearch();
    $("#term-title").textContent = ">_ " + label(s);
    setNote(s.note || "");
    try { s.fit.fit(); } catch {}
    if (s.lazy) { s.lazy = false; connect(s); } // sayfa yenilemesinden sonra yeniden bağlanan oturum
    renderBars();
    s.term.focus();
  }

  function minimize() { modal().classList.remove("open"); renderBars(); }

  function disposeSession(s, kill) {
    s.closed = true;
    try { s.es && s.es.close(); } catch {}
    try { s.ro && s.ro.disconnect(); } catch {}
    if (kill && s.id) post("/api/term/close", { id: s.id }).catch(() => {});
    try { s.term && s.term.dispose(); } catch {}
    if (s.host) s.host.remove();
  }

  function closeSession(s) {
    disposeSession(s, true);
    sessions = sessions.filter(x => x !== s);
    if (active === s) {
      active = null;
      if (sessions.length && modalOpen()) { show(sessions[sessions.length - 1]); return; }
      modal().classList.remove("open"); setNote("");
    }
    renderBars();
  }

  async function open(server) {
    if (typeof Terminal === "undefined" || typeof FitAddon === "undefined") { alert(tr("term_lib_missing", "xterm.js yüklenemedi.")); return; }
    if (sessions.length >= MAX) {
      const s0 = active || sessions[0]; show(s0); const m = tr("term_max", "En fazla 4 terminal oturumu açık olabilir; birini kapatın."); setNote(m); setTimeout(() => { if ($("#term-note").textContent === m) setNote(s0.note || ""); }, 4000); return;
    }
    const s = { id: "", server, term: null, fit: null, es: null, ro: null, host: null, closed: false, queue: "", sending: false, unread: false, built: false, opened: false, lazy: false, note: "" };
    sessions.push(s);
    show(s);
    let res;
    try {
      const r = await post("/api/term/open", { server, rows: s.term.rows, cols: s.term.cols });
      if (!r.ok) throw new Error((await r.text()).trim() || r.status);
      res = await r.json();
    } catch (e) {
      s.closed = true; s.note = "✖ " + e.message; if (active === s) setNote(s.note); renderBars();
      return;
    }
    if (!sessions.includes(s)) { post("/api/term/close", { id: res.id }).catch(() => {}); return; } // açılırken kapatıldı
    s.id = res.id; connect(s); renderBars();
    if (active === s && modalOpen()) s.term.focus();
  }

  async function adopt() { // sayfa yenilenmişse sunucuda yaşayan oturumları çubuğa geri getir
    try {
      const r = await fetch("/api/term/list", { headers: { "X-Requested-With": "sshpocket" } });
      if (!r.ok) return;
      for (const t of (await r.json()) || []) {
        if (t.done || sessions.length >= MAX || sessions.some(x => x.id === t.id)) continue;
        sessions.push({ id: String(t.id), server: String(t.server), term: null, fit: null, es: null, ro: null, host: null, closed: false, queue: "", sending: false, unread: false, built: false, opened: false, lazy: true, note: "" });
      }
      renderBars();
    } catch { /* çubuk boş kalır */ }
  }

  document.addEventListener("click", e => {
    if (e.target.closest("[data-term-min]")) minimize();
    else if (e.target.closest("[data-term-max]")) toggleMax();
    else if (e.target.closest("[data-term-close]") && active) closeSession(active);
  });
  window.addEventListener("keylab:langchange", () => renderBars());
  initSearchUi();
  initQuick();
  adopt();

  window.KeyLabTerminal = { open, close: () => active && closeSession(active), minimize };
})();
