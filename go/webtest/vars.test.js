"use strict";
/* Değişken düzenleme penceresi DAVRANIŞ testleri (refaktör 5c): açma, denetim türleri, süzgeçler, düzenle/kaydet/onay/sıfırla,
   kaydedince yan etkiler (sunucu özeti, katalog, karşılaştırma önbelleği). Taşımadan ÖNCE eski kodda da geçer. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const V = (o) => ({ name: "x", type: "string", default: "", value: "", set: false, required: false, group: "G", description: "açıklama", ...o });
const VARS = [
  V({ name: "v_str", description: "metin değişkeni", group: "Zaman", default: "UTC", placeholder: "x" }),
  V({ name: "v_bool", type: "bool", default: "true", group: "Zaman", description: "mantıksal" }),
  V({ name: "v_enum", type: "enum", values: ["docker", "nginx"], default: "docker", group: "Seçim", description: "seçim" }),
  V({ name: "v_int", type: "int", default: "5", min: "1", max: "9", set: true, value: "7", group: "Seçim", description: "sayı" }),
  V({ name: "v_num0", type: "number", default: "1", group: "Seçim", min: "", max: null, description: "min boş" }),
  V({ name: "v_list", type: "list[string]", default: "", required: true, group: "SSH", description: "liste", warnIf: "boşsa tehlikeli" }),
  V({ name: "v_nogrp", type: "string", default: "", group: "", description: "gruplanmamış" }),
  V({ name: "v_xss", type: "string", default: '"><b>', description: "<i>x</i>", group: "SSH" }),
];
const posts = [];
const varsApi = (over = {}) => ({
  "/api/home": mixedHome,
  "/api/vars": (u, { method, body }) => {
    if (method === "POST") { posts.push(body); return over.post ? over.post(body) : { ok: true }; }
    return { server: u.searchParams.get("server"), package: u.searchParams.get("package"), vars: over.vars || VARS };
  },
  ...over.o,
});
const open = async (h, srv = "web1", pkg = "utility") => { await h.click(`[data-tsrv="${srv}"]`); await h.click(`#srv-tabs [data-tab="${pkg}"], [data-ptab="${pkg}"]`).catch(() => {}); };
const isOpen = h => h.$("#varsmodal").classList.contains("open");
const names = h => h.$$("#vars-list .vrow").map(r => r.dataset.n);
const row = (h, n) => h.$(`#vars-list .vrow[data-n="${n}"]`);
const txt = s => s.textContent.replace(/\s+/g, " ").trim();
async function boot1(over, hash = "#s=web1&p=utility") { posts.length = 0; const h = await boot({ hash, overrides: varsApi(over) }); await h.click("#srv-body [data-vars]"); return h; }
const typeIn = async (h, n, v) => { const c = row(h, n).querySelector("[data-ctl]"); c.value = v; c.dispatchEvent(new h.win.Event("input", { bubbles: true })); await h.settle(); };

test("değişkenler: açılış (başlık, alt başlık, istek, sayaç, gruplar) ve kapatma", async () => {
  const h = await boot1();
  assert.ok(isOpen(h)); assert.equal(h.$("#vars-title").textContent, "Değişkenler · Utility"); assert.equal(h.$("#vars-sub").textContent, "Sunucu: web1 · servers/web1/utility.vars");
  const get = h.calls.filter(c => c.path.startsWith("/api/vars") && c.method === "GET").pop(); assert.equal(get.path, "/api/vars?server=web1&package=utility");
  assert.equal(h.$("#vars-count").textContent, "8 / 8"); assert.deepEqual(h.$$("#vars-list .vgroup").map(g => g.textContent), ["Zaman", "Seçim", "SSH", "Diğer"]);
  assert.deepEqual(names(h), ["v_str", "v_bool", "v_enum", "v_int", "v_num0", "v_list", "v⁠_xss".replace("⁠", ""), "v_nogrp"].sort((a, b) => names(h).indexOf(a) - names(h).indexOf(b)));
  await h.click("#vars-x"); assert.ok(!isOpen(h)); await h.click("#srv-body [data-vars]"); assert.ok(isOpen(h)); await h.click("#vars-close"); assert.ok(!isOpen(h));
  assert.deepEqual(h.errors, []); h.close();
});

test("değişkenler: denetim türleri (bool/enum/sayı/metin/liste) ve ilk değerler", async () => {
  const h = await boot1();
  const sel = n => row(h, n).querySelector("select[data-ctl]"), inp = n => row(h, n).querySelector("input[data-ctl]");
  assert.deepEqual([...sel("v_bool").options].map(o => o.value), ["true", "false"]); assert.equal(sel("v_bool").value, "true", "varsayılan 'true' seçili");
  assert.deepEqual([...sel("v_enum").options].map(o => o.value), ["docker", "nginx"]); assert.equal(sel("v_enum").value, "docker");
  assert.equal(inp("v_int").type, "number"); assert.equal(inp("v_int").value, "7", "ayarlıysa değer"); assert.equal(inp("v_int").min, "1"); assert.equal(inp("v_int").max, "9"); assert.equal(inp("v_int").getAttribute("step"), "any");
  assert.equal(inp("v_num0").hasAttribute("min"), false); assert.equal(inp("v_num0").hasAttribute("max"), false);
  assert.equal(inp("v_str").type, "text"); assert.equal(inp("v_str").value, "UTC", "ayarsızsa varsayılan"); assert.equal(inp("v_str").placeholder, "UTC"); assert.equal(inp("v_str").getAttribute("spellcheck"), "false"); assert.equal(inp("v_str").getAttribute("autocomplete"), "off");
  assert.equal(inp("v_list").getAttribute("aria-describedby"), "vl"); assert.equal(inp("v_str").hasAttribute("aria-describedby"), false);
  assert.equal(inp("v_xss").value, '"><b>'); assert.equal(h.$("#vars-list b"), null, "değerler kaçışlanır"); assert.match(row(h, "v_xss").querySelector(".vdesc").textContent, /<i>x<\/i>/);
  assert.deepEqual(h.errors, []); h.close();
});

test("değişkenler: rozetler, uyarı eşiği, 'varsayılana dön' yalnız ayarlıda, varsayılan metni", async () => {
  const h = await boot1();
  assert.match(txt(row(h, "v_int").querySelector(".vname")), /v_int ayarlı int/); assert.match(txt(row(h, "v_list").querySelector(".vname")), /gerekli/); assert.match(txt(row(h, "v_str").querySelector(".vname")), /varsayılan string/);
  assert.ok(row(h, "v_int").querySelector("[data-reset]")); assert.equal(row(h, "v_str").querySelector("[data-reset]"), null);
  assert.match(txt(row(h, "v_list")), /⚠ uyarı eşiği: boşsa tehlikeli/); assert.doesNotMatch(txt(row(h, "v_str")), /uyarı eşiği/);
  assert.match(txt(row(h, "v_str").querySelector(".row-flex")), /varsayılan: UTC/); assert.match(txt(row(h, "v_list").querySelector(".row-flex")), /varsayılan yok/);
  assert.equal(row(h, "v_str").querySelector("[data-save]").disabled, true);
  const d = row(h, "v_str").querySelector(".vdesc"); await h.click(d); assert.ok(d.classList.contains("open")); await h.click(d); assert.ok(!d.classList.contains("open"));
  assert.deepEqual(h.errors, []); h.close();
});

test("değişkenler: gerekli rozeti yalnız varsayılan yokken", async () => {
  const h = await boot1({ vars: [V({ name: "a", required: true, default: "d" }), V({ name: "b", required: true, default: "" }), V({ name: "c", required: true, set: true, value: "z" })] });
  assert.doesNotMatch(txt(row(h, "a").querySelector(".vname")), /gerekli/); assert.match(txt(row(h, "b").querySelector(".vname")), /gerekli/); assert.match(txt(row(h, "c").querySelector(".vname")), /ayarlı/); h.close();
});

test("değişkenler: arama (ad/açıklama/grup), 'yalnız ayarlılar', sayaç; yeniden açınca süzgeçler sıfırlanır", async () => {
  const h = await boot1();
  await h.type("#vars-q", "  METIN "); assert.deepEqual(names(h), ["v_str"]); assert.equal(h.$("#vars-count").textContent, "1 / 8");
  await h.type("#vars-q", "ssh"); assert.deepEqual(names(h).sort(), ["v_list", "v_xss"], "grup adıyla aranır");
  await h.type("#vars-q", "mantıksal"); assert.deepEqual(names(h), ["v_bool"], "açıklamayla aranır");
  await h.type("#vars-q", "yokyok"); assert.match(h.$("#vars-list").textContent, /Eşleşen değişken yok\./); assert.equal(h.$("#vars-count").textContent, "0 / 8");
  await h.type("#vars-q", ""); const set = h.$("#vars-set"); set.checked = true; set.dispatchEvent(new h.win.Event("change", { bubbles: true })); await h.settle();
  assert.deepEqual(names(h), ["v_int"]); assert.equal(h.$("#vars-count").textContent, "1 / 8");
  await h.type("#vars-q", "yokyok"); await h.click("#vars-close"); await h.click("#srv-body [data-vars]");
  assert.equal(h.$("#vars-q").value, ""); assert.equal(h.$("#vars-set").checked, false); assert.equal(names(h).length, 8);
  assert.deepEqual(h.errors, []); h.close();
});

test("değişkenler: yükleme sırasında 'yükleniyor…', hata → bildirim kutusu, null liste → boş", async () => {
  let release; const gate = new Promise(r => { release = r; });
  let h = await boot({ hash: "#s=web1&p=utility", overrides: { "/api/home": mixedHome, "/api/vars": async u => { if (h && h.__gate) await gate; return { vars: VARS }; } } });
  h.__gate = true; h.$("#srv-body [data-vars]").dispatchEvent(new h.win.MouseEvent("click", { bubbles: true })); await new Promise(r => setImmediate(r));
  assert.match(h.$("#vars-list").textContent, /yükleniyor…/); release(); await h.settle(); assert.equal(names(h).length, 8); h.close();
  h = await boot1({ o: { "/api/vars": { status: 500, body: "bozuk" } } }); assert.match(h.$("#vars-list .notice.danger").textContent, /Yüklenemedi/); h.close();
  h = await boot1({ o: { "/api/vars": () => ({ vars: null }) } }); assert.match(h.$("#vars-list").textContent, /Eşleşen değişken yok/); assert.equal(h.$("#vars-count").textContent, "0 / 0"); h.close();
});

test("değişkenler: düzenleyince kirli işareti ve Kaydet açılır; eski değere dönünce kapanır", async () => {
  const h = await boot1();
  const save = n => row(h, n).querySelector("[data-save]");
  await typeIn(h, "v_str", "Europe/Istanbul"); assert.ok(row(h, "v_str").classList.contains("dirty")); assert.equal(save("v_str").disabled, false);
  await typeIn(h, "v_str", "UTC"); assert.ok(!row(h, "v_str").classList.contains("dirty")); assert.equal(save("v_str").disabled, true);
  const s = row(h, "v_bool").querySelector("select"); s.value = "false"; s.dispatchEvent(new h.win.Event("change", { bubbles: true })); assert.equal(save("v_bool").disabled, false);
  assert.deepEqual(h.errors, []); h.close();
});

test("değişkenler: kaydet → POST gövdesi, bildirim, liste ve sunucu özeti yenilenir, karşılaştırma önbelleği düşer", async () => {
  const h = await boot1();
  let inval = 0, loads = []; const C = h.win.KeyLabCompare; const oi = C.invalidate, ol = C.load; C.invalidate = () => { inval++; return oi(); }; C.load = (...a) => { loads.push(a); return ol(...a); };
  const get = () => h.calls.filter(c => c.path.startsWith("/api/vars") && c.method === "GET").length, srvGet = () => h.calls.filter(c => c.path.startsWith("/api/server?name=web1")).length;
  const g0 = get(), s0 = srvGet();
  await typeIn(h, "v_str", "Europe/Istanbul"); await h.click(row(h, "v_str").querySelector("[data-save]"));
  assert.deepEqual(posts, [{ server: "web1", package: "utility", name: "v_str", value: "Europe/Istanbul", action: "set", ack: false }]);
  assert.match(h.$("#toast").textContent, /v_str kaydedildi/); assert.equal(get(), g0 + 1, "liste yeniden okunur"); assert.equal(srvGet(), s0 + 1, "sunucu özeti yeniden okunur"); assert.equal(inval, 1); assert.deepEqual(loads, [], "karşılaştırma sayfasında değilken yüklenmez");
  assert.ok(isOpen(h)); assert.deepEqual(h.errors, []); h.close();
});

test("değişkenler: Enter kaydeder (değişiklik yoksa kaydetmez)", async () => {
  const h = await boot1();
  const c = row(h, "v_str").querySelector("[data-ctl]"); const key = () => c.dispatchEvent(new h.win.KeyboardEvent("keydown", { key: "Enter", bubbles: true, cancelable: true }));
  key(); await h.settle(); assert.equal(posts.length, 0, "kirli değilken Enter bir şey yapmaz");
  await typeIn(h, "v_str", "Asia/Tokyo"); key(); await h.settle(); assert.equal(posts.length, 1); assert.equal(posts[0].value, "Asia/Tokyo");
  h.close();
});

test("değişkenler: onay gerektiren kayıt (409 needAck) iki adımda; düzenleyince onay sıfırlanır", async () => {
  let n = 0;
  const h = await boot1({ post: b => (b.ack ? { ok: true } : { status: 409, body: { needAck: true, error: "Bu ayar SSH'ı kesebilir." } }) });
  const btn = () => row(h, "v_str").querySelector("[data-save]");
  await typeIn(h, "v_str", "a"); await h.click(btn());
  assert.equal(btn().textContent, "Onayla ve kaydet"); assert.equal(btn().disabled, false); const m = row(h, "v_str").querySelector(".vmsg"); assert.ok(m.classList.contains("warn")); assert.equal(m.textContent, "⚠ Bu ayar SSH'ı kesebilir. Yine de kaydetmek için tekrar tıklayın.", "sondaki nokta kırpılıp yeniden eklenir");
  await typeIn(h, "v_str", "b"); assert.equal(btn().textContent, "Kaydet", "düzenleme onayı sıfırlar"); assert.equal(m.textContent, "");
  await h.click(btn()); assert.equal(btn().textContent, "Onayla ve kaydet"); await h.click(btn());
  assert.deepEqual(posts.map(p => [p.value, p.ack]), [["a", false], ["b", false], ["b", true]]); assert.match(h.$("#toast").textContent, /v_str kaydedildi/);
  assert.deepEqual(h.errors, []); h.close();
});

test("değişkenler: kayıt hatası mesajı; mesaj yoksa 'kaydedilemedi'; düğme yeniden açılır", async () => {
  let body = { error: "geçersiz değer" };
  const h = await boot1({ post: () => ({ status: 400, body }) });
  const btn = () => row(h, "v_str").querySelector("[data-save]"), msg = () => row(h, "v_str").querySelector(".vmsg");
  await typeIn(h, "v_str", "a"); await h.click(btn()); assert.equal(msg().textContent, "✖ geçersiz değer"); assert.ok(msg().classList.contains("err")); assert.equal(btn().disabled, false);
  body = {}; await h.click(btn()); assert.equal(msg().textContent, "✖ kaydedilemedi");
  assert.equal(posts.length, 2); assert.deepEqual(h.errors, []); h.close();
});

test("değişkenler: 'varsayılana dön' (unset) başarıyla ve hatayla", async () => {
  let fail = false;
  const h = await boot1({ post: () => (fail ? { status: 500, body: { error: "yazılamadı" } } : { ok: true }) });
  const rs = () => row(h, "v_int").querySelector("[data-reset]");
  fail = true; await h.click(rs()); assert.equal(row(h, "v_int").querySelector(".vmsg").textContent, "✖ yazılamadı"); assert.equal(rs().disabled, false);
  fail = false; await h.click(rs()); assert.deepEqual(posts.map(p => [p.name, p.action, p.value]), [["v_int", "unset", ""], ["v_int", "unset", ""]]); assert.match(h.$("#toast").textContent, /v_int varsayılana döndü/);
  fail = true; await h.click(rs()); // yenilenen satırda yeniden dene
  h.close();
});

test("değişkenler: kayıttan sonra katalog değerleri yeniden okunur (kurulum sekmesi)", async () => {
  const h = await boot({ hash: "#s=web1&p=kurulum", overrides: varsApi({ vars: fixture("vars-web1-kurulum").vars }) });
  const get = () => h.calls.filter(c => c.path.startsWith("/api/vars") && c.method === "GET").length;
  posts.length = 0; await h.click("#srv-body [data-kvars-all], #srv-body [data-vars], #srv-body [data-kvars]");
  const c = row(h, "kurulum_yazilim") ? "kurulum_yazilim" : names(h)[0]; const before = get();
  const ctl = row(h, c).querySelector("[data-ctl]"); ctl.value = ctl.tagName === "SELECT" ? [...ctl.options].pop().value : "zz"; ctl.dispatchEvent(new h.win.Event(ctl.tagName === "SELECT" ? "change" : "input", { bubbles: true })); await h.settle();
  await h.click(row(h, c).querySelector("[data-save]"));
  assert.equal(get() - before, 2, "pencere listesi + katalog kartları (catSrv sıfırlandığı için) yeniden okunur"); assert.deepEqual(h.errors, []); h.close();
});

test("değişkenler: kaydı karşılaştırma sayfasındayken yapmak karşılaştırmayı yeniden yükler (önbeleksiz)", async () => {
  const h = await boot1();
  const C = h.win.KeyLabCompare; let loads = []; const ol = C.load; C.load = (...a) => { loads.push(a); return ol(...a); };
  h.$("#view-compare").hidden = true; // görünüm adı appState.view; karşılaştırmaya geç
  await h.click('[data-nav="compare"]'); loads.length = 0; assert.ok(isOpen(h), "pencere sayfa değişse de açık kalır");
  await typeIn(h, "v_str", "q"); await h.click(row(h, "v_str").querySelector("[data-save]"));
  assert.deepEqual(loads, [[false]]); h.close();
});
