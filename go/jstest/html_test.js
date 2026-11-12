// `node jstest/html_test.js` — js/core/html.js davranış testi (Go testi tarafından çalıştırılır; node yoksa atlanır)
const assert = require("assert");
const { html, raw, esc, isSafe, setHtml } = require("../web/js/core/html.js");
const s = x => String(x);

// 1) değerler varsayılan kaçışlanır
assert.strictEqual(s(html`<b>${"<script>alert(1)</script>"}</b>`), "<b>&lt;script&gt;alert(1)&lt;/script&gt;</b>");
// 2) öznitelik kırma denemesi (çift tırnaklı öznitelikte güvenli)
assert.strictEqual(s(html`<a title="${'x" onmouseover="alert(1)'}">`), '<a title="x&quot; onmouseover=&quot;alert(1)">');
assert.strictEqual(s(html`<i>${"a'b&c"}</i>`), "<i>a&#39;b&amp;c</i>");
// 3) sabit parçalar olduğu gibi kalır
assert.strictEqual(s(html`<p class="x">ö</p>`), '<p class="x">ö</p>');
// 4) iç içe html`` yeniden kaçışlanmaz; dizi öğeleri aynı kuralı izler
assert.strictEqual(s(html`<ul>${["a", "<b>"].map(x => html`<li>${x}</li>`)}</ul>`), "<ul><li>a</li><li>&lt;b&gt;</li></ul>");
assert.strictEqual(s(html`${["<x>", html`<y>`]}`), "&lt;x&gt;<y>");
// 5) düz dize (kullanıcı verisi) asla güvenli sayılmaz; raw yalnız bilerek
assert.strictEqual(s(html`${"<i>x</i>"}`), "&lt;i&gt;x&lt;/i&gt;");
assert.strictEqual(s(html`${raw("<i>x</i>")}`), "<i>x</i>");
// 6) null / undefined / false boş; 0 ve "0" görünür
assert.strictEqual(s(html`[${null}${undefined}${false}]`), "[]");
assert.strictEqual(s(html`[${0}${"0"}]`), "[00]");
assert.strictEqual(s(html`${false && html`<b>x</b>`}${true && html`<b>y</b>`}`), "<b>y</b>");
// 7) SafeHtml taklidi: aynı şekilli düz nesne güvenli sayılmaz
assert.strictEqual(s(html`${{ s: "<b>", toString() { return "<b>"; } }}`), "&lt;b&gt;");
assert.strictEqual(isSafe({ s: "x" }), false);
assert.strictEqual(isSafe(html`x`), true);
// 8) setHtml düz dize kabul etmez
const el = { innerHTML: "" };
assert.throws(() => setHtml(el, "<b>x</b>"), TypeError);
assert.strictEqual(el.innerHTML, "");
setHtml(el, html`<b>${"<"}</b>`);
assert.strictEqual(el.innerHTML, "<b>&lt;</b>");
// 9) esc, eski esc() ile aynı çıktıyı verir (geçişte davranış farkı olmasın)
const old = s => String(s ?? "").replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
for (const v of ["", "a<b>&\"'", null, undefined, 12, "ş<ü>"]) assert.strictEqual(esc(v), old(v));
// 10) SafeHtml değiştirilemez
assert.throws(() => { "use strict"; html`x`.s = "y"; }, TypeError);
console.log("ok");
