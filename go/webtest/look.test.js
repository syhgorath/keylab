"use strict";
/* Yüzey görünümü (Düz / Cam + Aurora) testleri: seçim, saklama, geçersiz değer, ayar düğmeleri ve tasarim.css koruma kuralları
   (opak yedekler, -webkit- eşleri). Görsel doğruluk jsdom ile ölçülemez; ekran görüntüsüyle ayrıca denetlenir. */
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { boot, fixture } = require("./harness");

const mixedHome = () => { const h = fixture("home"); h.servers.forEach((s, i) => { s.checked = true; s.up = i !== 1; s.conn = s.up ? "up" : "down"; }); return h; };
const base = { "/api/home": mixedHome };
const attr = h => h.doc.documentElement.getAttribute("data-look");
const pressed = h => h.$$("#set-look button").filter(b => b.getAttribute("aria-pressed") === "true").map(b => b.dataset.v);

test("görünüm: varsayılan düz (öznitelik yok); Cam + Aurora seçilince öznitelik, saklama ve işaret; düze dönünce kalkar", async () => {
  const h = await boot({ hash: "#v=settings", overrides: base });
  assert.equal(attr(h), null); assert.deepEqual(pressed(h), ["flat"]); assert.equal(h.win.localStorage.getItem("keylab.look"), null);
  await h.click('#set-look button[data-v="glass"]'); assert.equal(attr(h), "glass"); assert.deepEqual(pressed(h), ["glass"]); assert.equal(h.win.localStorage.getItem("keylab.look"), "glass");
  await h.click('#set-look button[data-v="flat"]'); assert.equal(attr(h), null); assert.deepEqual(pressed(h), ["flat"]); assert.equal(h.win.localStorage.getItem("keylab.look"), "flat");
  assert.deepEqual(h.errors, []); h.close();
});

test("görünüm: kayıtlı tercih açılışta uygulanır; geçersiz değer yok sayılır", async () => {
  let h = await boot({ hash: "#v=settings", overrides: base, storage: { "keylab.look": "glass" } }); assert.equal(attr(h), "glass"); assert.deepEqual(pressed(h), ["glass"]); h.close();
  h = await boot({ hash: "#v=settings", overrides: base, storage: { "keylab.look": "neon" } }); assert.equal(attr(h), null); assert.deepEqual(pressed(h), ["flat"]); h.close();
});

test("görünüm: tema ve vurgu rengiyle bağımsız çalışır (öznitelikler birbirini ezmez)", async () => {
  const h = await boot({ hash: "#v=settings", overrides: base });
  await h.click('#set-look button[data-v="glass"]'); await h.click('#set-theme button[data-v="dark"]');
  assert.equal(attr(h), "glass"); assert.equal(h.doc.documentElement.getAttribute("data-theme"), "dark"); await h.click('#set-theme button[data-v="auto"]'); assert.equal(attr(h), "glass"); h.close();
});

/* ---------------- CSS koruma kuralları ---------------- */
const css = fs.readFileSync(path.join(__dirname, "..", "web", "tasarim.css"), "utf8");
const glassCss = css.slice(css.indexOf("Cam + Aurora"));

test("css: tüm cam kuralları :root[data-look=\"glass\"] kapsamında (düz görünümü etkilemez)", () => {
  const rules = glassCss.split("\n").filter(l => /^[^\s@/}].*\{/.test(l) || /^:root\[data-look/.test(l));
  assert.ok(rules.length >= 8);
  for (const sel of glassCss.match(/^(?::root\[data-look="glass"\][^{]*|\s+:root\[data-look="glass"\][^{]*)\{/gm) || []) assert.match(sel, /data-look="glass"/);
  // cam bloğunda kapsamsız (data-look'suz) .card/.modal/.topbar kuralı olmamalı
  const open = glassCss.replace(/\/\*[\s\S]*?\*\//g, "").split("\n").filter(l => /^(\.card|\.modal|\.topbar|\.sidebar|\.ctx|body)\b/.test(l.trim()) && l.includes("{") && !l.includes("data-look"));
  assert.deepEqual(open, []);
});

test("css: her backdrop-filter'ın -webkit- eşi var; opak yedekler (desteksiz, saydamlığı azalt, yüksek karşıtlık) tanımlı", () => {
  const plain = (glassCss.match(/(?<!-webkit-)backdrop-filter\s*:/g) || []).length, webkit = (glassCss.match(/-webkit-backdrop-filter\s*:/g) || []).length;
  assert.ok(plain >= 6); assert.equal(plain, webkit);
  assert.match(glassCss, /@supports not \(\(backdrop-filter:blur\(1px\)\) or \(-webkit-backdrop-filter:blur\(1px\)\)\)/);
  assert.match(glassCss, /@media \(prefers-reduced-transparency:reduce\),\(forced-colors:active\)/);
  const reduced = glassCss.slice(glassCss.indexOf("prefers-reduced-transparency")); assert.match(reduced, /backdrop-filter:none/); assert.match(reduced, /background-image:none/);
});

test("css: kart yüzeyi en az %72 opak (içerik okunurluğu); [hidden] kuralı dosyanın sonunda", () => {
  const m = glassCss.match(/\.card\{\s*background:color-mix\(in oklab,var\(--surface\) (\d+)%,transparent\)/); assert.ok(m, ".card cam arka planı"); assert.ok(Number(m[1]) >= 72);
  const lines = css.trimEnd().split("\n"); assert.match(lines[lines.length - 1], /^\[hidden\]\{display:none !important\}/);
});
