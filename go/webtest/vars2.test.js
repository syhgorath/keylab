"use strict";
/* Değişken düzenleme ek testleri (refaktör 5c): mutasyon turunda kaçanlar — grup kapsamı/başlık, URL kaçışı, bool büyük harf,
   Diğer grubu, X-Requested-With başlığı, kayıt sonrası sunucu durumu, kaydırma korunması, needAck olmayan 409, sıfırlama hata metni. */
const test = require("node:test");
const assert = require("node:assert/strict");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const V = o => ({ name: "x", type: "string", default: "", value: "", set: false, required: false, group: "G", description: "açıklama", ...o });
const posts = [];
const api = (vars, post) => ({
  "/api/home": mixedHome,
  "/api/vars": (u, { method, body }) => { if (method === "POST") { posts.push(body); return post ? post(body) : { ok: true }; } return { vars }; },
});
const row = (h, n) => h.$(`#vars-list .vrow[data-n="${n}"]`);
const typeIn = async (h, n, v) => { const c = row(h, n).querySelector("[data-ctl]"); c.value = v; c.dispatchEvent(new h.win.Event("input", { bubbles: true })); await h.settle(); };

test("değişkenler: grup kapsamı — kurulum kartından açınca başlık ve liste yalnız o grubu gösterir", async () => {
  const vars = fixture("vars-web1-kurulum").vars;
  const h = await boot({ hash: "#s=web1&p=kurulum", overrides: api(vars) });
  const b = h.$$("#srv-body [data-kvars]").find(x => x.dataset.kvars); assert.ok(b, "tek gruplu uygulama kartı düğmesi"); const g = b.dataset.kvars;
  await h.click(b);
  assert.equal(h.$("#vars-title").textContent, "Değişkenler · Kurulum · " + g);
  const inGroup = vars.filter(v => v.group === g).length; assert.equal(h.$("#vars-count").textContent, `${inGroup} / ${inGroup}`); assert.ok(inGroup < vars.length);
  assert.deepEqual(h.errors, []); h.close();
});

test("değişkenler: sunucu adı URL'de kaçışlanır", async () => {
  const h = await boot({ hash: "#s=" + encodeURIComponent("a&b c") + "&p=utility", overrides: { ...api([V({ name: "a" })]), "/api/home": () => { const x = mixedHome(); x.servers[0].name = "a&b c"; return x; }, "/api/server": u => { const d = fixture("server-web1"); d.server.name = u.searchParams.get("name"); return d; } } });
  await h.click("#srv-body [data-vars]");
  assert.deepEqual(h.calls.filter(c => c.path.startsWith("/api/vars")).map(c => c.path), ["/api/vars?server=a%26b%20c&package=utility"]); h.close();
});

test("değişkenler: bool büyük/küçük harf fark etmez; grupsuzlar 'Diğer' altında toplanır", async () => {
  const h = await boot({ hash: "#s=web1&p=utility", overrides: api([V({ name: "b", type: "bool", set: true, value: "False", default: "true", group: "" }), V({ name: "c", group: "" }), V({ name: "d", group: "Z" })]) });
  await h.click("#srv-body [data-vars]");
  assert.equal(row(h, "b").querySelector("select").value, "false");
  assert.deepEqual(h.$$("#vars-list .vgroup").map(g => g.textContent), ["Diğer", "Z"]);
  const kids = []; for (let e = h.$$("#vars-list .vgroup")[0].nextElementSibling; e && !e.classList.contains("vgroup"); e = e.nextElementSibling) kids.push(e.dataset.n);
  assert.deepEqual(kids, ["b", "c"], "ikisi de Diğer grubunda"); h.close();
});

test("değişkenler: kayıt isteği X-Requested-With başlığı taşır", async () => {
  const h = await boot({ hash: "#s=web1&p=utility", overrides: api([V({ name: "a" })]) });
  const orig = h.win.fetch, seen = []; h.win.fetch = (u, o) => { seen.push([String(u), o]); return orig(u, o); };
  await h.click("#srv-body [data-vars]"); await typeIn(h, "a", "q"); await h.click(row(h, "a").querySelector("[data-save]"));
  const p = seen.find(([u, o]) => u === "/api/vars" && o && o.method !== "GET" && o.body); assert.ok(p); assert.equal(p[1].headers["X-Requested-With"], "sshpocket"); h.close();
});

test("değişkenler: kayıttan sonra sunucu özeti yeni veriyle çizilir; kaydırma konumu korunur", async () => {
  let n = 0;
  const h = await boot({ hash: "#s=web1&p=utility", overrides: { ...api([V({ name: "a" })]), "/api/server": u => { const d = fixture("server-web1"); if (n++ > 0) d.server.platform = "YENI-PLATFORM"; return d; } } });
  await h.click("#srv-body [data-vars]"); assert.ok(!h.$("#srv-body").textContent.includes("YENI-PLATFORM"));
  const par = h.$("#vars-list").parentElement; const sets = []; Object.defineProperty(par, "scrollTop", { get: () => 321, set: v => sets.push(v), configurable: true });
  await typeIn(h, "a", "q"); await h.click(row(h, "a").querySelector("[data-save]"));
  assert.ok(h.doc.body.innerHTML.includes("YENI-PLATFORM") || h.$("#srv-body").textContent.includes("YENI-PLATFORM"), "sunucu durumu güncellendi");
  assert.deepEqual(sets, [321], "yeniden çizimden sonra eski kaydırma konumu geri yazılır"); h.close();
});

test("değişkenler: needAck olmayan 409 hata sayılır (onay istemez); sıfırlama hatasında varsayılan metin", async () => {
  const h = await boot({ hash: "#s=web1&p=utility", overrides: api([V({ name: "a" }), V({ name: "b", set: true, value: "1" })], b => (b.action === "unset" ? { status: 500, body: {} } : { status: 409, body: { error: "çakışma" } })) });
  await h.click("#srv-body [data-vars]"); await typeIn(h, "a", "q"); await h.click(row(h, "a").querySelector("[data-save]"));
  assert.equal(row(h, "a").querySelector("[data-save]").textContent, "Kaydet"); assert.equal(row(h, "a").querySelector(".vmsg").textContent, "✖ çakışma");
  await h.click(row(h, "b").querySelector("[data-reset]")); assert.equal(row(h, "b").querySelector(".vmsg").textContent, "✖ sıfırlanamadı"); h.close();
});
