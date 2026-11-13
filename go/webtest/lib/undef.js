"use strict";
/* Statik "tanımsız başvuru" denetleyicisi (no-undef). acorn ile ayrıştırır, kapsam (scope) zincirini kendisi kurar.
   AMAÇ: refaktörde bir tanımlayıcı taşınıp başka bir yerdeki kullanımı açıkta kalırsa (ör. refaktör 2c'deki `wiz`) bunu ÇALIŞTIRMADAN,
   yalnızca kaynağa bakarak yakalamak. Testler yalnız çalışan yolları görür; bu denetleyici her satırı görür.
   BİLEREK BASİT: TDZ (tanımdan önce kullanım) ve blok içi görünürlük sızıntıları denetlenmez; yalnız "bu ad hiçbir kapsamda tanımlı değil"
   sorusunu cevaplar (aşırı hoşgörülü, asla yanlış alarm vermemeye yakın). Dışa açık: findUndefined(kod, {globals}). */
const acorn = require("acorn");

// ECMAScript yerleşikleri + tarayıcı genel adları. Yeni bir meşru genel ad gerekirse BURAYA eklenir (her ekleme gözle incelenir).
const BUILTINS = new Set(`
  undefined NaN Infinity globalThis Object Function Array String Number Boolean Symbol BigInt Math JSON Date RegExp Error TypeError RangeError
  SyntaxError ReferenceError EvalError URIError AggregateError Promise Map Set WeakMap WeakSet WeakRef Proxy Reflect Intl ArrayBuffer SharedArrayBuffer
  DataView Int8Array Uint8Array Uint8ClampedArray Int16Array Uint16Array Int32Array Uint32Array Float32Array Float64Array BigInt64Array BigUint64Array
  parseInt parseFloat isNaN isFinite encodeURIComponent decodeURIComponent encodeURI decodeURI escape unescape eval queueMicrotask structuredClone
  window self document location history navigator localStorage sessionStorage fetch console setTimeout setInterval clearTimeout clearInterval
  requestAnimationFrame cancelAnimationFrame requestIdleCallback alert confirm prompt print innerWidth innerHeight devicePixelRatio screen scrollTo
  getComputedStyle getSelection matchMedia performance atob btoa CSS URL URLSearchParams Blob File FileReader FormData Headers Request Response
  AbortController AbortSignal TextEncoder TextDecoder crypto
  Node Element HTMLElement HTMLInputElement HTMLButtonElement HTMLSelectElement HTMLTextAreaElement DOMParser DocumentFragment ShadowRoot
  Event CustomEvent KeyboardEvent MouseEvent InputEvent FocusEvent PointerEvent MessageEvent ErrorEvent PromiseRejectionEvent EventTarget
  MutationObserver ResizeObserver IntersectionObserver EventSource WebSocket Image Audio Option NodeFilter Range Selection
  Terminal FitAddon SearchAddon WebLinksAddon
`.split(/\s+/).filter(Boolean));

class Scope {
  constructor(parent, node) { this.parent = parent; this.node = node; this.names = new Set(); }
  has(n) { for (let s = this; s; s = s.parent) if (s.names.has(n)) return true; return false; }
}

const isFn = n => n.type === "FunctionDeclaration" || n.type === "FunctionExpression" || n.type === "ArrowFunctionExpression";
const isNode = v => v && typeof v === "object" && typeof v.type === "string";

// bağlama deseninden (let/const/var/parametre/catch) tanımlanan adlar
function patternNames(p, out) {
  if (!p) return out;
  switch (p.type) {
    case "Identifier": out.add(p.name); break;
    case "ObjectPattern": p.properties.forEach(x => patternNames(x.type === "RestElement" ? x.argument : x.value, out)); break;
    case "ArrayPattern": p.elements.forEach(x => patternNames(x, out)); break;
    case "AssignmentPattern": patternNames(p.left, out); break;
    case "RestElement": patternNames(p.argument, out); break;
    default: break; // MemberExpression (atama hedefi): ad tanımlamaz
  }
  return out;
}

// işlev/program gövdesindeki `var` adları (iç işlevlere inmeden)
function varNames(node, out) {
  if (!isNode(node)) return;
  if (isFn(node)) return;
  if (node.type === "VariableDeclaration" && node.kind === "var") node.declarations.forEach(d => patternNames(d.id, out));
  for (const k of Object.keys(node)) {
    const v = node[k];
    if (Array.isArray(v)) v.forEach(x => varNames(x, out)); else if (isNode(v)) varNames(v, out);
  }
}

// bir deyim listesinin blok düzeyi tanımları (let/const/class/function/import)
function blockNames(stmts, out) {
  for (let s of stmts) {
    if (s.type === "ExportNamedDeclaration" && s.declaration) s = s.declaration;
    if (s.type === "ExportDefaultDeclaration" && s.declaration && (s.declaration.type === "FunctionDeclaration" || s.declaration.type === "ClassDeclaration") && s.declaration.id) s = s.declaration;
    if (s.type === "VariableDeclaration" && s.kind !== "var") s.declarations.forEach(d => patternNames(d.id, out));
    else if ((s.type === "FunctionDeclaration" || s.type === "ClassDeclaration") && s.id) out.add(s.id.name);
    else if (s.type === "ImportDeclaration") s.specifiers.forEach(sp => out.add(sp.local.name));
  }
}

// analyze: findUndefined'ın ayrıntılı hâli. Her başvuru için TANIMLANDIĞI kapsamı da döndürür (refs[].scope; yerleşik/genel ise null).
// Kod dönüşümleri (ör. refaktör 3b: modül düzeyi `let` değişkenlerini paylaşılan nesneye taşımak) bunu kullanır.
function findUndefined(code, opts = {}) { return analyze(code, opts).found; }

function analyze(code, { globals = [], file = "<kod>" } = {}) {
  const opts = { ecmaVersion: "latest", locations: true, allowHashBang: true };
  let ast;
  try { ast = acorn.parse(code, { ...opts, sourceType: "module" }); } catch (e) { ast = acorn.parse(code, { ...opts, sourceType: "script" }); }
  const extra = new Set(globals), found = [], refs = [], scopes = [];
  const mk = (parent, node) => { const sc = new Scope(parent, node); scopes.push(sc); return sc; };

  const ref = (id, scope) => {
    const n = id.name;
    for (let sc = scope; sc; sc = sc.parent) if (sc.names.has(n)) { refs.push({ id, name: n, scope: sc }); return; }
    if (BUILTINS.has(n) || extra.has(n)) { refs.push({ id, name: n, scope: null }); return; }
    found.push({ file, line: id.loc.start.line, col: id.loc.start.column + 1, name: n });
  };

  // desendeki ifadeleri (varsayılan değerler, hesaplanmış anahtarlar) gez; bağlanan adların kendisi başvuru DEĞİLDİR
  function visitPattern(p, scope) {
    if (!p) return;
    switch (p.type) {
      case "Identifier": break;
      case "ObjectPattern": p.properties.forEach(x => { if (x.type === "RestElement") visitPattern(x.argument, scope); else { if (x.computed) visit(x.key, scope); visitPattern(x.value, scope); } }); break;
      case "ArrayPattern": p.elements.forEach(x => visitPattern(x, scope)); break;
      case "AssignmentPattern": visitPattern(p.left, scope); visit(p.right, scope); break;
      case "RestElement": visitPattern(p.argument, scope); break;
      default: visit(p, scope); // MemberExpression vb.
    }
  }
  // atama hedefi: adlar BAŞVURUDUR (tanımsız bir adı atamak da hatadır)
  function visitTarget(p, scope) {
    if (!p) return;
    switch (p.type) {
      case "Identifier": ref(p, scope); break;
      case "ObjectPattern": p.properties.forEach(x => { if (x.type === "RestElement") visitTarget(x.argument, scope); else { if (x.computed) visit(x.key, scope); visitTarget(x.value, scope); } }); break;
      case "ArrayPattern": p.elements.forEach(x => visitTarget(x, scope)); break;
      case "AssignmentPattern": visitTarget(p.left, scope); visit(p.right, scope); break;
      case "RestElement": visitTarget(p.argument, scope); break;
      default: visit(p, scope);
    }
  }

  function visitFunction(fn, scope) {
    const fs = mk(scope, fn);
    if (fn.type === "FunctionExpression" && fn.id) fs.names.add(fn.id.name);
    if (fn.type !== "ArrowFunctionExpression") fs.names.add("arguments");
    fn.params.forEach(p => patternNames(p, fs.names));
    fn.params.forEach(p => visitPattern(p, fs));
    if (fn.body.type === "BlockStatement") {
      varNames(fn.body, fs.names); blockNames(fn.body.body, fs.names);
      fn.body.body.forEach(s => visit(s, fs));
    } else visit(fn.body, fs);
  }

  function visit(node, scope) {
    if (!isNode(node)) return;
    switch (node.type) {
      case "Identifier": ref(node, scope); return;
      case "FunctionDeclaration": case "FunctionExpression": case "ArrowFunctionExpression": visitFunction(node, scope); return;
      case "ClassDeclaration": case "ClassExpression": {
        const cs = mk(scope, node); if (node.id) cs.names.add(node.id.name);
        visit(node.superClass, scope);
        node.body.body.forEach(m => {
          if (m.type === "StaticBlock") { const bs = mk(cs, m); varNames({ type: "BlockStatement", body: m.body }, bs.names); blockNames(m.body, bs.names); m.body.forEach(s => visit(s, bs)); return; }
          if (m.computed) visit(m.key, cs);
          if (m.value) visit(m.value, cs);
        });
        return;
      }
      case "BlockStatement": { const bs = mk(scope, node); blockNames(node.body, bs.names); node.body.forEach(s => visit(s, bs)); return; }
      case "ForStatement": { const fs = mk(scope, node); if (node.init && node.init.type === "VariableDeclaration" && node.init.kind !== "var") node.init.declarations.forEach(d => patternNames(d.id, fs.names)); visit(node.init, fs); visit(node.test, fs); visit(node.update, fs); visit(node.body, fs); return; }
      case "ForInStatement": case "ForOfStatement": {
        const fs = mk(scope, node);
        if (node.left.type === "VariableDeclaration") { if (node.left.kind !== "var") node.left.declarations.forEach(d => patternNames(d.id, fs.names)); node.left.declarations.forEach(d => visitPattern(d.id, fs)); }
        else visitTarget(node.left, fs);
        visit(node.right, fs); visit(node.body, fs); return;
      }
      case "SwitchStatement": { visit(node.discriminant, scope); const ss = mk(scope, node); node.cases.forEach(c => blockNames(c.consequent, ss.names)); node.cases.forEach(c => { visit(c.test, ss); c.consequent.forEach(s => visit(s, ss)); }); return; }
      case "CatchClause": { const cs = mk(scope, node); patternNames(node.param, cs.names); visitPattern(node.param, cs); visit(node.body, cs); return; }
      case "VariableDeclaration": node.declarations.forEach(d => { visitPattern(d.id, scope); visit(d.init, scope); }); return;
      case "MemberExpression": visit(node.object, scope); if (node.computed) visit(node.property, scope); return;
      case "Property": if (node.computed) visit(node.key, scope); if (node.shorthand && node.value.type === "Identifier") node.value.shorthand = true; visit(node.value, scope); return;
      case "AssignmentExpression": visitTarget(node.left, scope); visit(node.right, scope); return;
      case "UnaryExpression": if (node.operator === "typeof" && node.argument.type === "Identifier") return; visit(node.argument, scope); return;
      case "LabeledStatement": visit(node.body, scope); return;
      case "BreakStatement": case "ContinueStatement": case "MetaProperty": case "ImportDeclaration": case "TemplateElement": case "PrivateIdentifier": return;
      case "ExportNamedDeclaration": if (node.declaration) visit(node.declaration, scope); else if (!node.source) node.specifiers.forEach(sp => ref(sp.local, scope)); return;
      case "ExportDefaultDeclaration": visit(node.declaration, scope); return;
      default:
        for (const k of Object.keys(node)) {
          if (k === "type" || k === "loc" || k === "start" || k === "end") continue;
          const v = node[k];
          if (Array.isArray(v)) v.forEach(x => visit(x, scope)); else if (isNode(v)) visit(v, scope);
        }
    }
  }

  const top = mk(null, ast);
  varNames(ast, top.names); blockNames(ast.body, top.names);
  ast.body.forEach(s => visit(s, top));
  return { found, refs, scopes, ast };
}

module.exports = { findUndefined, analyze, BUILTINS };
