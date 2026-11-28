/* KeyLab — Paket ayar kartları ve ayar penceresi (#setmodal): component.json "settings" / "setting_cards" bildirimi.
   Kart yalnız durum özetini gösterir; "Ayarlar" penceresi istenen değeri düzenler (Kaydet yalnız bu bilgisayardaki .vars dosyasını yazar) ve altında
   sunucudan SON OKUNAN değeri gösterir. Durumu arka uç hesaplar (pkgsettings.go).
   Tek tık: "Kontrol et" okumayı, "Kur" / "Uygula" / "Yükselt" önizleme → eylem → kontrol zincirini doğrudan gömülü terminalde açar (ayrı onay penceresi
   yok; parola ve gereken onaylar terminalde yazılır; jobs.js). Terminal açıkken bu pencere gizlenir, kapanınca geri gelir. Testler: go/webtest/ayar_kart.test.js */
import { $, $$, esc, json, toast, ago, localTime } from "../lib/util.js";
import { appState } from "../state.js";

const ST = { match: ["ok", "✔ uyumlu"], drift: ["warn", "⚠ sapma"], unknown: ["plain", "bilinmiyor"], absent: ["plain", "sunucuda yok"], unmanaged: ["plain", "yönetilmiyor"], unreadable: ["plain", "okunamaz"], na: ["plain", "uygulanmaz"] };
const RISK = { "access-loss": ["danger", "erişimi kesebilir"], "restarts-service": ["warn", "servisi yeniden başlatır"], "time-jump": ["warn", "saati sıçratır"], "data-loss": ["danger", "veriyi etkiler"] };
const uniq = l => [...new Set(l)];
// seçenek değerinin yanında paketin açıklaması ("agdan — ağdaki herkes"); değer değişmez
const valueText = (x, v) => (x.valueLabels && Object.hasOwn(x.valueLabels, v) ? `${v} — ${x.valueLabels[v]}` : v);
const chips = list => list.map(v => `<span class="chip">${esc(v)}</span>`).join("") || '<span class="muted">—</span>';
const risks = x => (x.risk || []).filter(r => RISK[r]).map(r => `<span class="badge ${RISK[r][0]}" title="${esc(x.riskNote || "")}">${RISK[r][1]}</span>`).join("");
const since = iso => { const v = ago(iso); return v === "şimdi" ? v : `${v} önce`; };
// sunucuda karşılığı olmayan ayar (okuma bildirilmemiş): yalnız uygulama sırasında kullanılan seçenek. Bu platformda okunamayan (notlu) ayar tabloda kalır.
const isOption = x => x.status === "unreadable" && !x.note;

// Kartlar: paketin bildirdiği sırayla "setting_cards"; kartı olmayan ayarlar değişken grubuna göre ayrı kart olur.
export function cardsOf(p) {
  const all = p.settings || [], out = [];
  for (const c of p.settingCards || []) {
    const rows = all.filter(x => x.card === c.id);
    if (rows.length) out.push({ id: c.id, label: c.label || c.id, description: c.description || "", rows, readAction: c.readAction || "", health: c.health || [], note: c.note || "", stale: !!c.stale });
  }
  const known = new Set(out.map(c => c.id));
  for (const x of all) {
    if (x.card && known.has(x.card)) continue;
    const id = "g:" + (x.group || "");
    let c = out.find(y => y.id === id);
    if (!c) out.push(c = { id, label: x.group || "Ayarlar", description: "", rows: [] });
    c.rows.push(x);
  }
  return out;
}

function cardBadge(c) {
  const rows = c.rows, n = st => rows.filter(x => x.status === st).length;
  const main = n("drift") ? `<span class="badge warn">${n("drift")} sapma</span>` : n("unknown") ? `<span class="badge plain">${n("unknown")} bilinmiyor</span>`
    : n("match") ? '<span class="badge ok">uyumlu</span>' : n("absent") ? '<span class="badge plain">sunucuda yok</span>' : n("unmanaged") ? '<span class="badge plain">yönetilmiyor</span>' : n("na") ? '<span class="badge plain">uygulanmaz</span>' : "";
  // ayarlar uyumlu olsa bile sağlık sorunu varsa kart yalnız yeşil görünmesin
  const ill = (c.health || []).some(h => h.state === "warn") ? '<span class="badge warn">⚠ dikkat</span>' : "";
  return main + ill + (c.stale || rows.some(x => x.stale) ? '<span class="badge warn" title="Bu okumadan sonra bu sunucuda değişiklik yapan bir eylem çalıştı.">güncel olmayabilir</span>' : "");
}

// sağlık özeti (kartın "summary" bildirimi): sonuç sözcükleri okunur yazılır, sayılara birim eklenir; durum arka uçtan gelir
const WORD = { yes: "evet", no: "hayır", evet: "evet", hayir: "hayır", durmus: "durmuş", active: "çalışıyor", inactive: "çalışmıyor", unknown: "bilinmiyor" };
function healthHtml(c) {
  if (!c.readAction) return "";
  if (!c.health.length) return `<div class="set-health small muted"><span>Sağlık bilgisi:</span> <span>${esc(c.note || "okuma bu değeri bildirmedi")}</span></div>`;
  return `<div class="set-health">${c.health.map(h => `<span class="set-hitem" data-st="${esc(h.state || "")}"><span class="muted">${esc(h.label)}</span> <b>${WORD[h.value] || esc(h.value)}</b>${h.unit && !WORD[h.value] ? ` <span class="muted">${esc(h.unit)}</span>` : ""}</span>`).join("")}</div>`;
}

// kartın okuma eylemleri (ayarların ve sağlık özetinin) ve "Son okuma" satırı
function readsOf(p, c) { return uniq([...c.rows.map(x => x.readAction), c.readAction].filter(Boolean)).map(id => p.actions.find(a => a.id === id)).filter(Boolean); }
function whenHtml(p, c) {
  const when = readsOf(p, c).map(a => `<span title="${esc(a.id)}${a.lastTime ? " · " + esc(localTime(a.lastTime)) : ""}">${a.lastTime ? esc(since(a.lastTime)) : "henüz okunmadı"}</span>`).join(" · ");
  return when ? `<span>Son okuma:</span> ${when}` : "";
}

// hooks: { openChain, openCheck — gömülü terminalde zincir / okuma; reload() — sunucu verisini yeniden al ve sayfayı çiz; saved() — değişken kaydedildi }
//   →  { cards(paket) → kart HTML'i, open(paket, kart), refresh() }
export function initPkgSettings(hooks) {
  const modal = $("#setmodal");
  let cur = null; // { srv, pkg, card, vars, ack: Set, closeAsked }
  const isOpen = () => modal.classList.contains("open");

  // app: katalogdaki bir uygulamanın yerini alan kart için { <kart kimliği>: { st: "on"|"off"|"?", badge, icon } } (kurulu bilgisi katalogdan)
  const cards = (p, app = {}) => cardsOf(p).map(c => {
    const a = Object.hasOwn(app, c.id) ? app[c.id] : null;
    // kurulu uygulamada kaldırma: paketin `<kart>_kaldir` eylemi (önizlemeli, e/h terminalde); onay düğmeyle verilmez
    const rm = a && a.st === "on" && (p.actions || []).find(x => x.id === c.id + "_kaldir" && x.review);
    return `<div class="card app-card set-app" data-setcard="${esc(c.id)}" data-st="${a ? a.st : "settings"}">
      <div class="app-top"><div class="app-ico" aria-hidden="true">${a ? a.icon : "⚙️"}</div><div class="app-titles"><div class="app-title"><span>${esc(c.label)}</span><span class="set-badges">${a ? a.badge : ""}${cardBadge(c)}</span></div><p class="app-desc">${esc(c.description)}</p></div></div>
      ${healthHtml(c)}<div class="app-foot"><span class="small muted">${whenHtml(p, c)}</span><div class="app-btns">${rm ? `<button class="btn sm dangerbtn" data-review="${esc(p.id)}|${esc(rm.id)}" title="Önce kaldırma önizlemesi koşar; terminalde e yazarsanız kaldırılır (Enter: hayır)">Kaldır</button>` : ""}<button class="btn sm primary" data-setopen="${esc(c.id)}">Ayarlar</button></div></div></div>`;
  }).join("");

  const pkgNow = () => appState.srv && (appState.srv.packages || []).find(x => x.id === cur.pkg);
  const cardNow = p => p && cardsOf(p).find(c => c.id === cur.card);

  const varOf = name => cur.vars.find(v => v.name === name);
  const valueOf = v => String((v.set ? v.value : v.default) ?? "");
  const ctlValue = el => (el.type === "checkbox" ? String(el.checked) : el.value.trim());
  const ctlBase = el => { const v = varOf(el.dataset.var); return el.type === "checkbox" ? String(valueOf(v).toLowerCase() === "true") : valueOf(v); };
  const dirty = () => $$("#set-body [data-var]").filter(el => ctlValue(el) !== ctlBase(el));

  function control(x) {
    const v = varOf(x.var);
    if (!v) return `<span class="small muted">değişken bulunamadı: <span class="mono">${esc(x.var)}</span></span>`;
    const val = valueOf(v), attr = `data-var="${esc(v.name)}" aria-label="${esc(x.label)}"`;
    if (v.type === "bool") return `<input type="checkbox" ${attr}${val.toLowerCase() === "true" ? " checked" : ""}>`;
    if (v.values && v.values.length) return `<select ${attr}>${v.values.map(o => `<option value="${esc(o)}"${o === val ? " selected" : ""}>${esc(valueText(x, o))}</option>`).join("")}</select>`;
    const type = /^(int|integer|number|float)$/.test(v.type) ? "number" : "text";
    return `<input type="${type}" ${attr} value="${esc(val)}" placeholder="${esc(v.default || "boş = dokunma")}" spellcheck="false" autocomplete="off">${/^list/.test(v.type) ? '<div class="small muted">Birden çok değeri virgülle ayırın.</div>' : ""}`;
  }
  function nameHtml(x) {
    const name = `<span class="set-name" title="${esc(x.var)}">${esc(x.label)}</span>`;
    if (!x.description && !x.riskNote) return name;
    return `<details class="set-info"><summary title="Açıklamayı göster">${name}<span class="set-i" aria-hidden="true">ⓘ</span></summary>
      <div class="small muted">${x.description ? `<div>${esc(x.description)}</div>` : ""}${x.riskNote ? `<div>${esc(x.riskNote)}</div>` : ""}</div></details>`;
  }

  function paintBody(c) {
    const fields = c.rows.filter(x => !isOption(x)), opts = c.rows.filter(isOption);
    $("#set-body").innerHTML = (c.readAction ? '<div class="set-field" id="set-health"></div>' : "") + fields.map(x => `<div class="set-field" data-set="${esc(x.id)}">
        <div class="set-fhead">${nameHtml(x)}${risks(x)}<span class="grow"></span><span data-live="state"></span></div>
        ${control(x)}<div data-live="have"></div></div>`).join("") +
      (opts.length ? `<div class="set-field set-optbox"><div class="small muted">Uygulama seçenekleri:</div>${opts.map(x => `<div class="set-opt" data-set="${esc(x.id)}">${control(x)}${nameHtml(x)}${risks(x)}</div>`).join("")}</div>` : "");
    $$("#set-body [data-var]").forEach(el => {
      let last = ctlValue(el); // "input"tan sonra kutudan çıkınca gelen aynı değerli "change" hiçbir şeyi değiştirmez
      const on = () => { if (ctlValue(el) === last) return; last = ctlValue(el); cur.ack.clear(); cur.closeAsked = false; say("", ""); paintLive(); };
      el.addEventListener("input", on); el.addEventListener("change", on);
    });
  }

  // yalnız içerik değiştiyse yazar: aksi hâlde tıklanmakta olan düğme (kutudan çıkış, 10 sn'lik tazeleme) yerinden sökülür ve tıklama kaybolur
  const painted = new WeakMap();
  const put = (el, html) => { if (painted.get(el) !== html) { painted.set(el, html); el.innerHTML = html; } };

  // sunucudan gelen kısımlar (durum, sunucudaki değer, düğmeler): kutulardaki yazılan değere dokunmadan yenilenir
  function paintLive() {
    const p = pkgNow(), c = cardNow(p);
    if (!c) return;
    put($("#set-sub"), `<span>Sunucu: ${esc(cur.srv)}</span>${whenHtml(p, c) ? " · " + whenHtml(p, c) : ""}`);
    if ($("#set-health")) put($("#set-health"), healthHtml(c));
    for (const x of c.rows) {
      const el = $$("#set-body .set-field[data-set]").find(f => f.dataset.set === x.id);
      if (!el) continue;
      const [cls, label] = ST[x.status] || ST.unknown;
      // uyumluysa istenen yazım gösterilir (sonuç satırı '/' gibi karakterleri '_' yapar)
      const have = x.status === "match" ? x.desired : x.actual;
      put($('[data-live="state"]', el), `<span class="badge ${cls}">${label}</span>`);
      put($('[data-live="have"]', el), `<div class="set-have"><span class="small muted">Sunucuda:</span>${chips((have || []).map(v => valueText(x, v)))}</div>${x.note ? `<div class="small muted">${esc(x.note)}</div>` : ""}`);
    }
    const changed = dirty().length > 0;
    const byId = id => p.actions.find(a => a.id === id);
    const managed = c.rows.some(x => x.status !== "unmanaged" && (x.desired || []).length);
    const applies = managed ? uniq(c.rows.flatMap(x => x.applyVia || [])).map(byId).filter(a => a && a.applicable) : [];
    const btn = (a, text, cls, attr, why) => (why ? `<button class="btn sm" disabled title="${esc(why)}">${text}</button>`
      : `<button class="btn sm ${cls}" ${attr} data-action="${esc(a.id)}" title="${esc(a.id)}">${text}</button>`);
    const reads = readsOf(p, c);
    put($("#set-reads"), reads.map(a => btn(a, reads.length > 1 ? `Kontrol et: ${esc(a.id)}` : "Kontrol et", "", "data-check",
      a.runnable || a.terminal ? "" : a.reason || "şimdi çalıştırılamaz")).join(""));
    const verb = a => (/_kur$/.test(a.id) ? "Kur" : /_yukselt$/.test(a.id) ? "Yükselt" : "Uygula");
    put($("#set-acts"), `<button class="btn sm${applies.length ? "" : " primary"}" data-save${changed ? "" : " disabled"}>${cur.ack.size ? "Onayla ve kaydet" : "Kaydet"}</button>` +
      applies.map(a => btn(a, applies.filter(b => verb(b) === verb(a)).length > 1 ? `${verb(a)}: ${esc(a.id)}` : verb(a), a.destructive ? "dangerbtn" : "primary", "data-chain",
        (a.missingVars || []).length ? `gerekli değişken eksik: ${a.missingVars.join(", ")}` : "")).join(""));
  }

  const say = (cls, text) => { const m = $("#set-msg"); m.className = "small " + cls; m.textContent = text; };
  const loadVars = async () => (await json(`/api/vars?server=${encodeURIComponent(cur.srv)}&package=${encodeURIComponent(cur.pkg)}`)).vars || [];

  async function open(pkg, card) {
    cur = { srv: appState.srvName, pkg, card, vars: [], ack: new Set(), closeAsked: false };
    const c = cardNow(pkgNow());
    if (!c) { cur = null; return; }
    $("#set-title").textContent = c.label; $("#set-sub").textContent = `Sunucu: ${cur.srv}`;
    $("#set-body").innerHTML = '<div class="small muted">yükleniyor…</div>'; put($("#set-reads"), ""); put($("#set-acts"), ""); say("", "");
    modal.classList.add("open");
    const mine = cur;
    try { const vars = await loadVars(); if (cur !== mine) return; cur.vars = vars; }
    catch (e) { if (cur === mine) $("#set-body").innerHTML = `<div class="notice danger"><span class="ico" aria-hidden="true">✕</span><div>Yüklenemedi: ${esc(e.message)}</div></div>`; return; }
    paintBody(c); paintLive();
  }

  // Kaydet: değişen değerleri sırayla yazar. Boş bırakılan değer satırı siler (paket varsayılanı). Uyarı eşiği (409 needAck) ikinci tıklamayla onaylanır.
  async function save() {
    const mine = cur;
    for (const el of dirty()) {
      const name = el.dataset.var, value = ctlValue(el), v = varOf(name);
      const action = value === "" ? "unset" : "set";
      if (action === "unset" && !v.set) { el.value = valueOf(v); continue; }
      let r, body;
      try {
        r = await fetch("/api/vars", { method: "POST", headers: { "X-Requested-With": "sshpocket" }, body: JSON.stringify({ server: cur.srv, package: cur.pkg, name, value, action, ack: cur.ack.has(name) }) });
        body = await r.json().catch(() => ({}));
      } catch (e) { if (cur === mine) say("err", `✖ ${name}: ${e.message}`); return false; }
      if (cur !== mine) return;
      if (!r.ok) {
        if (r.status === 409 && body.needAck) { cur.ack.add(name); say("warn", `⚠ ${String(body.error).replace(/[.;\s]+$/, "")}. Yine de kaydetmek için tekrar tıklayın.`); }
        else say("err", `✖ ${name}: ${body.error || "kaydedilemedi"}`);
        paintLive(); return false;
      }
      cur.ack.delete(name);
      if (action === "set") { v.value = value; v.set = true; } else { v.value = ""; v.set = false; el.value = valueOf(v); }
    }
    toast("Ayarlar kaydedildi (sunucu değişmedi)"); hooks.saved();
    try { const vars = await loadVars(); if (cur !== mine) return; cur.vars = vars; await hooks.reload(); } catch (e) { /* sonraki yenilemede toparlanır */ }
    if (cur !== mine) return;
    const c = cardNow(pkgNow());
    if (c) { paintBody(c); paintLive(); say("ok", "✔ Kaydedildi. Sunucuya yazmak için Kur / Uygula."); }
    return true;
  }

  // Kur / Uygula / Yükselt: kaydedilmemiş değer varsa önce kaydeder (uyarı onayı gerekiyorsa durur), sonra zinciri açar
  async function chain(action) {
    const mine = cur;
    if (dirty().length && !(await save())) return;
    if (cur === mine) hooks.openChain(mine.srv, mine.pkg, action);
  }

  const forceClose = () => { cur = null; modal.classList.remove("open", "behind"); };
  function close() {
    if (!cur) return forceClose();
    if (dirty().length && !cur.closeAsked) { cur.closeAsked = true; say("warn", "Kaydedilmemiş değişiklik var. Kapatmak için tekrar tıklayın."); return; }
    forceClose();
  }

  // sayfa yeniden çizildikçe (10 sn'lik tazeleme, iş bitişi) pencere de tazelenir
  function refresh() {
    if (!cur || !isOpen()) return;
    if (appState.srvName !== cur.srv || !cardNow(pkgNow())) return forceClose();
    if ($("#set-body [data-var]")) paintLive();
  }

  $("#set-x").addEventListener("click", close);
  $("#set-reads").addEventListener("click", e => {
    const b = e.target.closest("[data-check]");
    if (b && cur) hooks.openCheck(cur.srv, cur.pkg, b.dataset.action);
  });
  $("#set-acts").addEventListener("click", e => {
    if (!cur) return;
    const b = e.target.closest("[data-chain]");
    if (e.target.closest("[data-save]")) save(); else if (b) chain(b.dataset.action);
  });
  // başka bir pencere (onay, terminal, Python denetimi…) açıkken bu pencere gizlenir: iki pencere üst üste binmez
  const others = $$(".modal-back").filter(m => m !== modal);
  const covered = () => others.some(m => m.classList.contains("open"));
  const sync = () => modal.classList.toggle("behind", isOpen() && covered());
  const obs = new MutationObserver(sync);
  others.forEach(m => obs.observe(m, { attributes: true, attributeFilter: ["class"] }));
  // yakalama aşaması: üstteki pencerenin Esc işleyicisi onu kapatmadan ÖNCE bakılır; Esc bu pencereye yalnız üstte pencere yokken ulaşır
  document.addEventListener("keydown", e => { if (e.key === "Escape" && isOpen() && !covered()) close(); }, true);

  return { cards, open, refresh };
}
