"use strict";
/* KeyLab arayüz test kasası (jsdom). Gerçek index.html'i ve web/ altındaki betikleri, gerçek Go sunucusundan KAYDEDİLMİŞ API yanıtlarıyla
   (fixtures/) çalıştırır. Sabit saat, kontrollü zamanlayıcılar, sahte fetch/EventSource/xterm: sonuç her koşuda aynıdır.
   Bu bir DAVRANIŞ KİLİDİDİR: refaktör sırasında DOM çıktısı (anlık görüntü) değişmemelidir. Tarayıcıyı ve CSS'i test ETMEZ (yerleşim yok). */
// Saat dilimi SABİT: uygulama saatleri yerel saatle biçimler; anlık görüntüler makineden bağımsız olsun (CI UTC, geliştirici makinesi değil).
// jsdom/Date kullanılmadan ÖNCE atanmalıdır.
process.env.TZ = "UTC";
const fs = require("fs");
const path = require("path");
const { JSDOM, VirtualConsole } = require("jsdom");

const WEB = path.join(__dirname, "..", "web");
const FIX = path.join(__dirname, "fixtures");
const NOW = Date.UTC(2026, 10, 14, 10, 0, 0); // 2026-11-14T10:00:00Z

const fixture = n => JSON.parse(fs.readFileSync(path.join(FIX, n + ".json"), "utf8"));

// index.html'deki <script src> etiketleri. Tarayıcı anlamı: klasik betikler sırayla hemen, type="module" betikler AYRIK (sayfa ayrıştırılınca,
// sırayla) çalışır; kasa aynı sırayı uygular. vendor/ (xterm) gerçek değil, sahtesi kullanılır.
// (HTML düzenlemek için regex KULLANILMAZ: DOM sorgulanır. jsdom runScripts "outside-only" ile sayfadaki betikleri çalıştırmaz/yüklemez.)
function scriptList(doc) {
  return [...doc.querySelectorAll("script[src]")].map(el => ({ src: el.getAttribute("src"), module: el.getAttribute("type") === "module" })).filter(x => !x.src.startsWith("vendor/"));
}

// ES modülü: jsdom'un VM bağlamında vm.SourceTextModule ile çalıştırır (jsdom kendisi modül desteklemez). Her boot için TAZE örnek
// (modül durumu testler arasında sızmaz). Yalnız göreli içe aktarma; dosya yolları web/ altında çözülür.
async function runModule(dom, file) {
  const vm = require("vm");
  if (typeof vm.SourceTextModule !== "function") throw new Error("ES modülleri için node --experimental-vm-modules gerekir (npm test bunu verir)");
  const ctx = dom.getInternalVMContext(), cache = new Map();
  const load = abs => {
    if (!cache.has(abs)) cache.set(abs, new vm.SourceTextModule(fs.readFileSync(abs, "utf8"), { context: ctx, identifier: abs }));
    return cache.get(abs);
  };
  const root = load(file);
  await root.link((spec, ref) => {
    if (!spec.startsWith(".")) throw new Error("yalnız göreli içe aktarma desteklenir: " + spec);
    const abs = path.resolve(path.dirname(ref.identifier), spec);
    if (!abs.startsWith(WEB + path.sep)) throw new Error("web/ dışına içe aktarma: " + spec);
    return load(abs);
  });
  await root.evaluate();
}

// varsayılan sahte API. overrides: { "/api/yol": veri | (url, opts) => veri | {status, body} }
function makeRouter(overrides, calls) {
  const home = () => fixture("home");
  const table = {
    "/api/home": home, "/api/status": () => fixture("status"), "/api/jobs": () => fixture("jobs"), "/api/batch": () => fixture("batch"),
    "/api/ui-settings": () => fixture("ui-settings"), "/api/packages": () => fixture("packages"), "/api/about": () => fixture("about"),
    "/api/profiles": () => fixture("profiles"), "/api/logs": () => fixture("logs"), "/api/term/list": () => [],
    "/api/batch/options": () => ({ options: [], total: 0 }),
    "/api/updates": () => ({ status: 409, body: "fixture: indeks yok" }),
  };
  return async function route(url, opts = {}) {
    const u = new URL(url, "http://127.0.0.1:7777");
    const method = (opts.method || "GET").toUpperCase();
    let body = null; try { body = opts.body ? JSON.parse(opts.body) : null; } catch { body = opts.body; }
    calls.push({ method, path: u.pathname + u.search, body });
    const key = u.pathname;
    let h = overrides[u.pathname + u.search] ?? overrides[key];
    if (h === undefined) {
      if (key === "/api/server") h = () => fixture("server-" + u.searchParams.get("name"));
      else if (key === "/api/lynis") h = () => fixture("lynis-" + u.searchParams.get("server"));
      else if (key === "/api/vars") h = () => fixture(`vars-${u.searchParams.get("server")}-${u.searchParams.get("package")}`);
      else h = table[key];
    }
    if (h === undefined) throw new Error(`fixture yok: ${method} ${u.pathname}${u.search}`);
    let res = typeof h === "function" ? await h(u, { method, body }) : h;
    let status = 200, text, json;
    let hdrs = {}; if (res && typeof res === "object" && "status" in res && "body" in res) { status = res.status; hdrs = res.headers || {}; res = res.body; }
    if (typeof res === "string") text = res; else { json = res; text = JSON.stringify(res); }
    return { ok: status >= 200 && status < 300, status, json: async () => (json !== undefined ? json : JSON.parse(text)), text: async () => text, headers: { get: k => (k in hdrs ? hdrs[k] : null) } };
  };
}

// kontrollü zamanlayıcılar: hiçbiri kendiliğinden çalışmaz; testler runTimers() ile ilerletir
function makeTimers() {
  let id = 0, t = 0; const q = new Map();
  return {
    setTimeout: (fn, ms = 0) => { q.set(++id, { fn, at: t + ms, every: 0 }); return id; },
    setInterval: (fn, ms = 0) => { q.set(++id, { fn, at: t + Math.max(1, ms), every: Math.max(1, ms) }); return id; },
    clear: i => { q.delete(i); },
    run(ms) { const end = t + ms; for (;;) { let nx = null; for (const [k, v] of q) if (v.at <= end && (!nx || v.at < nx[1].at)) nx = [k, v]; if (!nx) break; const [k, v] = nx; t = v.at; if (v.every) v.at += v.every; else q.delete(k); v.fn(); } t = end; },
    pending: () => q.size,
  };
}

function fakeXterm(win) {
  class Terminal {
    constructor(o) { this.options = o; this.rows = 24; this.cols = 80; this.buffer = { active: { getLine: () => null } }; this.written = []; }
    loadAddon(a) { a.term = this; } open(el) { this.el = el; } write(d) { this.written.push(d); } focus() {} dispose() {} onData() {} onResize() {}
    attachCustomKeyEventHandler() {} registerLinkProvider() {} getSelection() { return ""; } clearSelection() {}
  }
  win.Terminal = Terminal;
  win.FitAddon = { FitAddon: class { fit() {} } };
  win.SearchAddon = { SearchAddon: class { findNext() { return true; } findPrevious() { return true; } } };
  win.WebLinksAddon = { WebLinksAddon: class {} };
}

// Yakalanmayan söz (promise) reddi: uygulama kodu async işlevlerde sessizce çökebilir (listener'dan çağrılan async işlev). Bu kasa onları
// test HATASI yapar: canlı her kasanın errors dizisine yazılır (kapanınca çıkar). Aksi hâlde "hatasız" görünen sayfa aslında yarım çizilmiş olabilir.
const liveErrors = new Set();
process.on("unhandledRejection", reason => { const m = "unhandledRejection: " + (reason && reason.stack || reason); for (const e of liveErrors) e.push(m); });

// opts: { hash, overrides, storage, lang }
async function boot(opts = {}) {
  const html = fs.readFileSync(path.join(WEB, "index.html"), "utf8");
  const errors = [], calls = [], vc = new VirtualConsole();
  liveErrors.add(errors);
  vc.on("jsdomError", e => errors.push("jsdom: " + (e.detail?.stack || e.message)));
  vc.on("error", (...a) => errors.push("console.error: " + a.join(" ")));
  const dom = new JSDOM(html, {
    url: "http://127.0.0.1:7777/" + (opts.hash || ""), runScripts: "outside-only", pretendToBeVisual: true, virtualConsole: vc,
  });
  const win = dom.window, doc = win.document;
  const timers = makeTimers();
  const RealDate = win.Date;
  win.Date = class extends RealDate { constructor(...a) { super(...(a.length ? a : [NOW])); } static now() { return NOW; } };
  // 0 ms'lik zamanlayıcılar (uygulama dinleyicileri "render sonrası" bağlamak için kullanır) hemen çalışır ve settle() onları bekler;
  // gecikmeli zamanlayıcılar (yoklama, odak, bildirim kapanışı) kontrollüdür: yalnız timers.run() ile ilerler.
  let inflight = 0, zero = 0, zid = 1e6; const zeroTimers = new Set();
  win.setTimeout = (fn, ms = 0, ...a) => {
    if (ms > 0) return timers.setTimeout(() => fn(...a), ms);
    const id = ++zid; zero++; zeroTimers.add(id);
    setImmediate(() => { zero--; if (zeroTimers.delete(id)) fn(...a); });
    return id;
  };
  win.setInterval = timers.setInterval;
  win.clearTimeout = win.clearInterval = i => { if (zeroTimers.delete(i)) { zero--; return; } timers.clear(i); };
  const route = makeRouter(opts.overrides || {}, calls);
  win.fetch = async (u, o) => { inflight++; try { return await route(u, o); } catch (e) { errors.push(String(e.message)); return { ok: false, status: 500, json: async () => ({}), text: async () => String(e.message) }; } finally { inflight--; } };
  win.EventSource = class { addEventListener() {} close() {} };
  win.ResizeObserver = class { observe() {} disconnect() {} unobserve() {} };
  win.matchMedia = () => ({ matches: false, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {} });
  win.CSS = { supports: () => false };
  win.print = () => {}; win.scrollTo = () => {}; win.alert = () => {};
  win.HTMLElement.prototype.scrollIntoView = () => {};
  win.URL.createObjectURL = () => "blob:fixture"; win.URL.revokeObjectURL = () => {};
  Object.defineProperty(win.navigator, "clipboard", { value: { writeText: async t => { win.__clip = (win.__clip || []).concat(t); } }, configurable: true });
  for (const [k, v] of Object.entries(opts.storage || {})) win.localStorage.setItem(k, v);
  if (opts.lang) win.localStorage.setItem("keylab.lang", opts.lang);
  fakeXterm(win);
  win.addEventListener("error", e => errors.push("window.error: " + e.message));

  const scripts = scriptList(doc);
  for (const { src } of scripts.filter(x => !x.module)) {
    const code = fs.readFileSync(path.join(WEB, src), "utf8");
    try { win.eval(code + "\n//# sourceURL=" + src); } catch (e) { errors.push(`betik ${src}: ${e.stack || e}`); }
  }
  for (const { src } of scripts.filter(x => x.module)) {
    try { await runModule(dom, path.join(WEB, src)); } catch (e) { errors.push(`modül ${src}: ${e.stack || e}`); }
  }
  const h = { dom, win, doc, calls, errors, timers };
  h.settle = async (n = 30) => { for (let i = 0; i < n; i++) { await new Promise(r => setImmediate(r)); if (!inflight && !zero && i > 3) { await new Promise(r => setImmediate(r)); if (!inflight && !zero) return; } } };
  h.$ = s => doc.querySelector(s); h.$$ = s => [...doc.querySelectorAll(s)];
  h.click = async s => { const el = typeof s === "string" ? doc.querySelector(s) : s; if (!el) throw new Error("öğe yok: " + s); el.dispatchEvent(new win.MouseEvent("click", { bubbles: true, cancelable: true })); await h.settle(); };
  h.type = async (s, v) => { const el = h.$(s); el.value = v; el.dispatchEvent(new win.Event("input", { bubbles: true })); await h.settle(); };
  h.close = () => { liveErrors.delete(errors); win.close(); };
  await h.settle();
  return h;
}

// DOM'u kararlı, okunur, satır tabanlı metne çevirir (öznitelikler sıralı; boşluklar sadeleşmiş; betik/yorum yok)
function snapshot(root) {
  const lines = [];
  const walk = (n, d) => {
    if (n.nodeType === 3) { const t = n.textContent.replace(/\s+/g, " ").trim(); if (t) lines.push("  ".repeat(d) + JSON.stringify(t)); return; }
    if (n.nodeType !== 1 || n.tagName === "SCRIPT") return;
    const at = [...n.attributes].map(a => `${a.name}=${JSON.stringify(a.value)}`).sort().join(" ");
    lines.push("  ".repeat(d) + "<" + n.tagName.toLowerCase() + (at ? " " + at : "") + ">");
    for (const c of n.childNodes) walk(c, d + 1);
  };
  walk(root, 0);
  return lines.join("\n") + "\n";
}

const SNAP = path.join(__dirname, "snapshots");
// golden dosyasıyla karşılaştırır; UPDATE_SNAPSHOTS=1 ile yazar
function expectSnapshot(assert, name, actual) {
  const f = path.join(SNAP, name + ".txt");
  if (process.env.UPDATE_SNAPSHOTS === "1" || !fs.existsSync(f)) { fs.mkdirSync(SNAP, { recursive: true }); fs.writeFileSync(f, actual); return; }
  const want = fs.readFileSync(f, "utf8");
  if (want !== actual) {
    const a = want.split("\n"), b = actual.split("\n"); let i = 0; while (i < a.length && a[i] === b[i]) i++;
    assert.fail(`anlık görüntü uyuşmuyor: ${name}\n  ilk fark satır ${i + 1}\n  beklenen: ${(a[i] || "").slice(0, 160)}\n  gerçek:   ${(b[i] || "").slice(0, 160)}\n  (bilerek değiştiyse: UPDATE_SNAPSHOTS=1 npm test)`);
  }
}

module.exports = { boot, snapshot, expectSnapshot, fixture, NOW };
