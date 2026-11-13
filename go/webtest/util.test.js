"use strict";
/* js/lib/util.js birim testleri (durumsuz yardımcılar). Anlık görüntüler bu dalları (şimdi/dk/sa, '<' kaçışı, istek başlığı) kapsamaz;
   burada doğrudan, sınır değerleriyle denetlenir. util.js yalnız çağrı anında document/window/fetch'e dokunur: Node'da import edilebilir. */
process.env.TZ = "UTC";
const test = require("node:test");
const assert = require("node:assert/strict");
const path = require("path");
const { pathToFileURL } = require("url");

const load = () => import(pathToFileURL(path.join(__dirname, "..", "web", "js", "lib", "util.js")).href);
const NOW = Date.UTC(2026, 10, 14, 10, 0, 0);
const withNow = async (fn) => { const real = Date.now; Date.now = () => NOW; try { return await fn(); } finally { Date.now = real; } };
const iso = secAgo => new Date(NOW - secAgo * 1000).toISOString();

test("esc: beş özel karakter kaçışlanır, null/undefined boş, sayı metne döner", async () => {
  const { esc } = await load();
  assert.equal(esc(`<a href="x" title='y'>&</a>`), "&lt;a href=&quot;x&quot; title=&#39;y&#39;&gt;&amp;&lt;/a&gt;");
  assert.equal(esc(null), ""); assert.equal(esc(undefined), ""); assert.equal(esc(0), "0"); assert.equal(esc(12), "12");
  assert.equal(esc("ş<ü>"), "ş&lt;ü&gt;"); assert.equal(esc("&amp;"), "&amp;amp;"); // çift kaçış: veri olduğu gibi gösterilir
});

test("ago: sınırlar (89 sn şimdi, 90 sn 2 dk, 1 sa, 1 gün) ve geçersiz tarih", async () => {
  const { ago } = await load();
  await withNow(() => {
    assert.equal(ago(iso(0)), "şimdi"); assert.equal(ago(iso(89)), "şimdi");
    assert.equal(ago(iso(90)), "2 dk"); assert.equal(ago(iso(3599)), "60 dk");
    assert.equal(ago(iso(3600)), "1 sa"); assert.equal(ago(iso(86399)), "24 sa");
    assert.equal(ago(iso(86400)), "1 gün"); assert.equal(ago(iso(5 * 86400)), "5 gün");
    assert.equal(ago(iso(-300)), "şimdi", "gelecekteki zaman negatif olmaz");
    assert.equal(ago(""), ""); assert.equal(ago("bozuk"), ""); assert.equal(ago(undefined), "");
  });
});

test("localTime: tr-TR biçimi; geçersiz girdi olduğu gibi döner", async () => {
  const { localTime } = await load();
  assert.equal(localTime("2026-11-10T09:00:00Z"), "10.11.2026 09:00:00");
  assert.equal(localTime("bozuk"), "bozuk");
});

test("isOk ve resHtml", async () => {
  const { isOk, resHtml } = await load();
  assert.equal(isOk("ok"), true); assert.equal(isOk("fail"), false); assert.equal(isOk(undefined), false);
  assert.match(resHtml(null), /kayıt yok/); assert.match(resHtml({ result: "ok" }), /res ok.*başarılı/);
  assert.match(resHtml({ result: "fail" }), /res fail.*başarısız/); assert.match(resHtml({ result: "başka" }), /res fail/);
});

test("api: GET/POST, X-Requested-With başlığı, gövde, 401 ve hata metni", async () => {
  const { api, json } = await load();
  const calls = [];
  const mk = (status, body) => ({ ok: status >= 200 && status < 300, status, text: async () => body, json: async () => JSON.parse(body) });
  let next = mk(200, '{"a":1}');
  globalThis.fetch = async (u, o) => { calls.push([u, o]); return next; };
  try {
    assert.deepEqual(await json("/x"), { a: 1 });
    assert.equal(calls[0][1].method, "GET"); assert.equal(calls[0][1].body, undefined);
    assert.equal(calls[0][1].headers["X-Requested-With"], "sshpocket", "CSRF başlığı her istekte gitmeli");
    await api("/y", { k: "v" });
    assert.equal(calls[1][1].method, "POST"); assert.equal(calls[1][1].body, '{"k":"v"}'); assert.equal(calls[1][1].headers["X-Requested-With"], "sshpocket");
    next = mk(401, ""); await assert.rejects(api("/z"), /oturum yok/);
    next = mk(409, "  bu sunucuda başka bir eylem çalışıyor \n"); await assert.rejects(api("/z"), /^Error: bu sunucuda başka bir eylem çalışıyor$/);
    next = mk(500, ""); await assert.rejects(api("/z"), /500/);
  } finally { delete globalThis.fetch; }
});

test("toast: metin yazılır, gösterilir, 2400 ms sonra gizlenir; yeni toast eski zamanlayıcıyı iptal eder", async () => {
  const { toast } = await load();
  const el = { textContent: "", style: { display: "none" } }, timers = [];
  globalThis.document = { querySelector: s => (s === "#toast" ? el : null) };
  const rs = globalThis.setTimeout, rc = globalThis.clearTimeout;
  globalThis.setTimeout = (fn, ms) => { timers.push({ fn, ms, cancelled: false }); return timers.length; };
  globalThis.clearTimeout = id => { if (id && timers[id - 1]) timers[id - 1].cancelled = true; };
  try {
    toast("merhaba"); assert.equal(el.textContent, "merhaba"); assert.equal(el.style.display, "block"); assert.equal(timers[0].ms, 2400);
    toast("ikinci"); assert.equal(timers[0].cancelled, true, "eski zamanlayıcı iptal"); assert.equal(el.textContent, "ikinci");
    timers[1].fn(); assert.equal(el.style.display, "none");
  } finally { globalThis.setTimeout = rs; globalThis.clearTimeout = rc; delete globalThis.document; }
});

test("$ ve $$: kök ve varsayılan belge", async () => {
  const { $, $$ } = await load();
  const root = { querySelector: s => "kök:" + s, querySelectorAll: s => new Set([s, s + "2"]) };
  assert.equal($("a", root), "kök:a"); assert.deepEqual($$("b", root), ["b", "b2"]);
  globalThis.document = { querySelector: s => "belge:" + s, querySelectorAll: () => [1, 2] };
  try { assert.equal($("c"), "belge:c"); assert.deepEqual($$("d"), [1, 2]); } finally { delete globalThis.document; }
});

test("t: i18n yoksa varsayılan, varsa onun çevirisi", async () => {
  const { t } = await load();
  globalThis.window = {}; // tarayıcıda window her zaman vardır
  try {
    assert.equal(t("anahtar", "varsayılan"), "varsayılan");
    globalThis.window = { KeyLabI18n: { t: (k, d) => `[${k}|${d}]` } };
    assert.equal(t("a", "b"), "[a|b]");
  } finally { delete globalThis.window; }
});
