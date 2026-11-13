"use strict";
/* Statik tanımsız-başvuru denetimi: web/ altındaki TÜM arayüz betikleri (vendor hariç). Bkz. lib/undef.js.
   Önce denetleyicinin KENDİSİ sınanır (yanlış alarm ve kaçırma), sonra gerçek kaynak taranır. */
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("fs");
const path = require("path");
const { findUndefined } = require("./lib/undef");

const names = code => findUndefined(code).map(x => x.name).sort();

// ---------------- denetleyicinin kendisi ----------------
test("undef: tanımsız adı yakalar (okuma, çağırma, atama, güncelleme, şablon ifadesi, yayılma)", () => {
  assert.deepEqual(names("foo();"), ["foo"]);
  assert.deepEqual(names("const a = bar + 1;"), ["bar"]);
  assert.deepEqual(names("baz = 3;"), ["baz"], "tanımsız adı atamak da hatadır");
  assert.deepEqual(names("qux++;"), ["qux"]);
  assert.deepEqual(names("const s = `x${tpl}y`;"), ["tpl"]);
  assert.deepEqual(names("f(...rest);"), ["f", "rest"]);
  assert.deepEqual(names("const o = { a: missing };"), ["missing"]);
  assert.deepEqual(names("const o = { shorthand };"), ["shorthand"], "kısa özellik değeri başvurudur");
  assert.deepEqual(names("const o = { [comp]: 1 };"), ["comp"]);
  assert.deepEqual(names("x?.y?.[z];"), ["x", "z"]);
  assert.deepEqual(names("[a, b] = [1, 2];"), ["a", "b"], "atama deseninde tanımsız adlar");
  assert.deepEqual(names("function f() { return inner; }"), ["inner"]);
});

test("undef: refaktör 2c hatasının aynısı (taşınan değişkenin başka yerdeki kullanımı)", () => {
  const code = `(function () {
    const wizModule = {};            // wiz başka dosyaya taşındı
    setInterval(() => { if (document.hidden || wiz.classList.contains("open")) return; }, 10000);
  })();`;
  assert.deepEqual(findUndefined(code).map(x => `${x.name}@${x.line}`), ["wiz@3"]);
});

test("undef: meşru kullanımlarda yanlış alarm VERMEZ", () => {
  const ok = [
    "function f() { return g(); } function g() {}",                                  // işlev bildirimi hoisting
    "f(); function f() {}",
    "const a = 1; { const b = a; console.log(b); }",
    "var v = 1; (function () { return v; })();",
    "function outer() { { var hoisted = 1; } return hoisted; }",                      // var blok dışına taşar
    "const f = (a, b = a) => a + b;",                                                  // varsayılan değer önceki parametreyi görür
    "const { x, y: [z, ...r], ...rest } = obj; use(x, z, r, rest); const obj = {}; function use() {}",
    "function f({ a, b = 2 }, [c, d], ...more) { return a + b + c + d + more.length; }",
    "try { risky(); } catch (e) { console.log(e); } function risky() {}",
    "try { 1; } catch ({ message }) { console.log(message); }",
    "for (let i = 0; i < 3; i++) console.log(i);",
    "for (const k in {}) console.log(k); for (const [a, b] of []) console.log(a, b);",
    "var i; for (i of []) { console.log(i); }",
    "switch (1) { case 1: { let s = 1; console.log(s); } default: let t = 2; console.log(t); }",
    "class A { static s = 1; #p = 2; m() { return A.s + this.#p; } static { const q = 1; console.log(q); } get g() { return 1; } set g(v) {} }",
    "const C = class Inner { m() { return Inner; } };",
    "const fe = function self(n) { return n ? self(n - 1) : 0; };",
    "function f() { return arguments.length; }",
    "outer: for (;;) { for (;;) { break outer; } }",
    "const o = { a: 1, 'b-c': 2, get d() { return 1; }, e() { return this.a; }, [`k${1}`]: 3 }; o.a.b.c;",
    "const t = typeof maybeUndefinedGlobal; if (typeof other === 'undefined') {}",    // typeof korumalı kullanım
    "async function f() { await Promise.all([]); for await (const x of []) console.log(x); } function* g() { yield 1; }",
    "import { a as b } from './x.js'; import c, * as d from './y.js'; console.log(b, c, d);",
    "export const e1 = 1; export function e2() {} export { e1 as renamed }; export default class {}",
    "const fn = function () { return new.target; }; console.log(import.meta.url);",
    "window.foo = 1; document.querySelector('x'); fetch('/'); new Map(); JSON.parse('1'); Math.max(1); Terminal; localStorage;",
    "let late; function use() { return late; } late = 1;",
    "const a = b => c => a + b + c;",
    "label: { break label; }",
  ];
  for (const c of ok) assert.deepEqual(findUndefined(c).map(x => x.name), [], c);
});

test("undef: kapsam sınırlarına saygı duyar (blok içi let dışarıdan görünmez, işlev içi tanım dışarı sızmaz)", () => {
  assert.deepEqual(names("{ let a = 1; } a;"), ["a"]);
  assert.deepEqual(names("function f() { const inner = 1; } inner;"), ["inner"]);
  assert.deepEqual(names("for (let i = 0; i < 1; i++) {} i;"), ["i"]);
  assert.deepEqual(names("try {} catch (e) {} e;"), ["e"]);
  assert.deepEqual(names("const a = () => { const x = 1; }; x;"), ["x"]);
  assert.deepEqual(names("const o = { m() { return unknown; } };"), ["unknown"]);
  assert.deepEqual(names("class A { m() { return nope; } }"), ["nope"]);
});

test("undef: konum bilgisi doğru; extra globals kabul edilir", () => {
  const f = findUndefined("\n\n  zip();", { file: "x.js" })[0];
  assert.deepEqual([f.file, f.line, f.col, f.name], ["x.js", 3, 3, "zip"]);
  assert.deepEqual(findUndefined("zip();", { globals: ["zip"] }), []);
});

// ---------------- gerçek kaynak ----------------
const WEB = path.join(__dirname, "..", "web");
const files = [];
(function walk(d) {
  for (const e of fs.readdirSync(d, { withFileTypes: true })) {
    const p = path.join(d, e.name);
    if (e.isDirectory()) { if (e.name !== "vendor") walk(p); } else if (e.name.endsWith(".js")) files.push(p);
  }
})(WEB);

test("undef: web/ altındaki bütün betikler ayrıştırılır ve en az 20 dosya taranır", () => {
  assert.ok(files.length >= 20, `dosya sayısı: ${files.length}`);
  for (const f of files) assert.doesNotThrow(() => findUndefined(fs.readFileSync(f, "utf8"), { file: f }), f);
});

// DOSYA BAŞINA istisnalar (genel değil): html.js, Node testlerinde de yüklenebilsin diye `typeof module` korumalı UMD kalıbı kullanır.
const ALLOW = { "js/core/html.js": ["module"] };
test("undef: web/ altında TANIMSIZ başvuru yok", () => {
  const all = files.flatMap(f => { const rel = path.relative(WEB, f); return findUndefined(fs.readFileSync(f, "utf8"), { file: rel, globals: ALLOW[rel] || [] }); });
  assert.deepEqual(all.map(x => `${x.file}:${x.line}:${x.col} ${x.name}`), []);
});

test("undef: gerçek app.js üzerinde mutasyon — bir tanımı silmek yakalanır", () => {
  const app = fs.readFileSync(path.join(WEB, "app.js"), "utf8");
  for (const [from, to, name] of [
    ["const matches = (...parts)", "const matchesX = (...parts)", "matches"],
    ['import { appState } from "./js/state.js";', 'import { appStateX } from "./js/state.js";', "appState"],
    ["async function load()", "async function loadX()", "load"],
  ]) {
    assert.ok(app.includes(from), `${from} app.js'te bulunamadı`);
    const hit = findUndefined(app.replace(from, to), { file: "app.js" }).map(x => x.name);
    assert.ok(hit.includes(name), `${name} silinince yakalanmalı: ${hit.slice(0, 5)}`);
  }
});
