/* Gömülü terminal (xterm.js): kayıtlı bir sunucuya ssh -tt, sunucu tarafı PTY (bkz. ../../term.go).
   Güvenlik: sunucudan gelen metin terminale YALNIZ xterm'in kendi ayrıştırıcısıyla yazılır; hata iletileri textContent ile gösterilir,
   terminale elle metin yazılırken yalnız sabit dizgeler ve sayılar kullanılır (kontrol karakterleri sızmaz). */
(function () {
  "use strict";
  const H = { "X-Requested-With": "sshpocket", "Content-Type": "application/json" };
  const post = (path, body) => fetch(path, { method: "POST", headers: H, body: JSON.stringify(body) });
  const b64ToBytes = s => Uint8Array.from(atob(s), c => c.charCodeAt(0));
  const tr = (k, d) => (window.KeyLabI18n ? window.KeyLabI18n.t(k, d) : d);

  let cur = null; // { id, server, term, fit, es, ro, closed, queue, sending }

  function $(s) { return document.querySelector(s); }
  function setNote(msg) { const n = $("#term-note"); n.textContent = msg || ""; n.hidden = !msg; }

  async function flushInput(s) {
    if (s.sending || !s.queue) return;
    s.sending = true;
    try {
      while (s.queue && !s.closed) {
        const data = s.queue; s.queue = "";
        const r = await post("/api/term/input", { id: s.id, data });
        if (!r.ok) { setNote(tr("term_input_fail", "Girdi iletilemedi: oturum kapanmış olabilir.")); break; }
      }
    } catch { setNote(tr("term_input_fail", "Girdi iletilemedi: oturum kapanmış olabilir.")); }
    s.sending = false;
  }

  function teardown(kill) {
    const s = cur; if (!s) return;
    cur = null; s.closed = true;
    try { s.es && s.es.close(); } catch {}
    try { s.ro && s.ro.disconnect(); } catch {}
    if (kill && s.id) post("/api/term/close", { id: s.id }).catch(() => {});
    try { s.term && s.term.dispose(); } catch {}
    $("#term-host").textContent = "";
  }

  function closeModal() { teardown(true); $("#termmodal").classList.remove("open"); }

  async function open(server) {
    if (typeof Terminal === "undefined" || typeof FitAddon === "undefined") { alert(tr("term_lib_missing", "xterm.js yüklenemedi.")); return; }
    teardown(true);
    $("#term-title").textContent = ">_ " + server;
    setNote("");
    const host = $("#term-host"); host.textContent = "";
    $("#termmodal").classList.add("open");

    const term = new Terminal({ cursorBlink: true, fontFamily: 'ui-monospace, Menlo, Consolas, monospace', fontSize: 13, scrollback: 5000,
      theme: { background: "#0e1116", foreground: "#d6dbe4" }, allowProposedApi: false });
    const fit = new FitAddon.FitAddon();
    term.loadAddon(fit); term.open(host);
    try { fit.fit(); } catch {}
    const s = cur = { id: "", server, term, fit, es: null, ro: null, closed: false, queue: "", sending: false };

    let res;
    try {
      const r = await post("/api/term/open", { server, rows: term.rows, cols: term.cols });
      if (!r.ok) throw new Error((await r.text()).trim() || r.status);
      res = await r.json();
    } catch (e) {
      if (cur === s) { setNote("✖ " + e.message); s.closed = true; }
      return;
    }
    if (cur !== s) { post("/api/term/close", { id: res.id }).catch(() => {}); return; } // açılırken modal kapatıldı
    s.id = res.id;

    term.onData(d => { s.queue += d; flushInput(s); });
    let rt; term.onResize(({ rows, cols }) => { clearTimeout(rt); rt = setTimeout(() => { if (!s.closed) post("/api/term/resize", { id: s.id, rows, cols }).catch(() => {}); }, 120); });
    s.ro = new ResizeObserver(() => { try { fit.fit(); } catch {} }); s.ro.observe(host);

    const es = s.es = new EventSource("/api/term/stream?id=" + encodeURIComponent(s.id));
    es.addEventListener("data", ev => { try { term.write(b64ToBytes(ev.data)); } catch {} });
    es.addEventListener("exit", ev => {
      s.closed = true; es.close();
      const code = String(parseInt(ev.data, 10) || 0); // yalnız sayı
      term.write("\r\n\x1b[90m[" + tr("term_exited", "oturum bitti") + ": " + code + "]\x1b[0m\r\n");
    });
    es.onerror = () => { if (!s.closed) { setNote(tr("term_conn_lost", "Akış koptu; oturum kapanmış olabilir.")); } };
    term.focus();
  }

  document.addEventListener("click", e => {
    if (e.target.closest("[data-term-close]")) closeModal();
  });

  window.KeyLabTerminal = { open, close: closeModal };
})();
