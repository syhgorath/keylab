/* KeyLab — Değişken düzenleme penceresi: paket değişkenlerini listeler, türüne göre denetim çizer, kaydeder/onaylatır/varsayılana döndürür.
   app.js'ten çıkarıldı (refaktör 5c), kod BİREBİR; dış bağlantılar hooks ile. Testler: go/webtest/vars.test.js */
import { $, $$, esc, json, toast } from "../lib/util.js";
import { appState } from "../state.js";
import { TAB_LABEL } from "../lib/labels.js";

// hooks: { renderServer() — sunucu sayfasını yeniden çiz; resetCatalog() — katalog kartlarındaki değerler yeniden okunsun }
//   →  { open(sunucu, paket, grup?), close(), cur(değişken) }
export function initVars(hooks) {
  const { renderServer, resetCatalog } = hooks;
  const varsm = $("#varsmodal");
  let vData = [], vSrv = "", vPkg = "", vQ = "", vOnlySet = false, vGroup = "";

  async function openVars(server, pkg, group) {
    vSrv = server; vPkg = pkg; vQ = ""; vOnlySet = false; vGroup = group || ""; // group: yalnız bu gruptaki değişkenler (uygulama kartı)
    $("#vars-q").value = ""; $("#vars-set").checked = false;
    $("#vars-title").textContent = "Değişkenler · " + (TAB_LABEL[pkg] || pkg) + (vGroup ? " · " + vGroup : "");
    $("#vars-sub").textContent = `Sunucu: ${server} · servers/${server}/${pkg}.vars`;
    $("#vars-list").innerHTML = '<div class="small muted">yükleniyor…</div>';
    varsm.classList.add("open");
    await loadVars();
  }
  async function loadVars() {
    try { vData = (await json(`/api/vars?server=${encodeURIComponent(vSrv)}&package=${encodeURIComponent(vPkg)}`)).vars || []; }
    catch (e) { $("#vars-list").innerHTML = `<div class="notice danger"><span class="ico" aria-hidden="true">✕</span><div>Yüklenemedi: ${esc(e.message)}</div></div>`; return; }
    paintVars();
  }
  const varCur = v => (v.set ? v.value : v.default);

  function varControl(v) {
    const cur = varCur(v);
    if (v.type === "bool") return `<select data-ctl>${["true", "false"].map(o => `<option value="${o}"${String(cur).toLowerCase() === o ? " selected" : ""}>${o}</option>`).join("")}</select>`;
    if (v.values && v.values.length) return `<select data-ctl>${v.values.map(o => `<option value="${esc(o)}"${o === cur ? " selected" : ""}>${esc(o)}</option>`).join("")}</select>`;
    if (/^(int|integer|number|float)$/.test(v.type)) return `<input data-ctl type="number" value="${esc(cur)}"${v.min !== "" && v.min != null ? ` min="${esc(v.min)}"` : ""}${v.max !== "" && v.max != null ? ` max="${esc(v.max)}"` : ""} step="any">`;
    return `<input data-ctl type="text" value="${esc(cur)}" placeholder="${esc(v.default)}" spellcheck="false" autocomplete="off"${/^list/.test(v.type) ? ' aria-describedby="vl"' : ""}>`;
  }

  function paintVars() {
    const q = vQ.toLowerCase(), scope = vData.filter(v => !vGroup || v.group === vGroup);
    const list = scope.filter(v => (!vOnlySet || v.set) && (!q || `${v.name} ${v.description} ${v.group}`.toLowerCase().includes(q)));
    $("#vars-count").textContent = `${list.length} / ${scope.length}`;
    const groups = {};
    list.forEach(v => (groups[v.group || "Diğer"] = groups[v.group || "Diğer"] || []).push(v));
    const row = v => `<div class="vrow" data-n="${esc(v.name)}">
      <div><div class="vname">${esc(v.name)} ${v.set ? '<span class="badge ok">ayarlı</span>' : v.required && !v.default ? '<span class="badge danger">gerekli</span>' : '<span class="badge plain">varsayılan</span>'} <span class="chip sj-11a508">${esc(v.type)}</span></div>
        <div class="vdesc small muted" title="Tamamını görmek için tıklayın">${esc(v.description)}</div>
        ${v.warnIf ? `<div class="small sj-0e2fcf">⚠ uyarı eşiği: ${esc(v.warnIf)}</div>` : ""}</div>
      <div class="vctl">${varControl(v)}
        <div class="row-flex"><button class="btn sm primary" data-save disabled>Kaydet</button>${v.set ? '<button class="btn sm ghost" data-reset title="Satırı siler; paket varsayılanına döner">Varsayılana dön</button>' : ""}<span class="small muted">${v.default !== "" ? `varsayılan: <span class="mono">${esc(v.default)}</span>` : "varsayılan yok"}</span></div>
        <div class="vmsg" role="status"></div></div></div>`;
    $("#vars-list").innerHTML = Object.entries(groups).map(([g, vs]) => `<div class="vgroup">${esc(g)}</div>${vs.map(row).join("")}`).join("") || '<div class="small muted">Eşleşen değişken yok.</div>';
    $$("#vars-list .vrow").forEach(rowEl => bindVarRow(rowEl));
  }

  function bindVarRow(rowEl) {
    const v = vData.find(x => x.name === rowEl.dataset.n), ctl = $("[data-ctl]", rowEl), save = $("[data-save]", rowEl), msg = $(".vmsg", rowEl);
    const say = (cls, t) => { msg.className = "vmsg " + cls; msg.textContent = t; };
    let ack = false;
    $(".vdesc", rowEl).addEventListener("click", e => e.currentTarget.classList.toggle("open"));
    const dirty = () => { const d = ctl.value !== varCur(v); rowEl.classList.toggle("dirty", d); save.disabled = !d; ack = false; save.textContent = "Kaydet"; if (d) say("", ""); };
    ctl.addEventListener("input", dirty); ctl.addEventListener("change", dirty);
    ctl.addEventListener("keydown", e => { if (e.key === "Enter" && !save.disabled) { e.preventDefault(); save.click(); } });
    const post = async (action, value) => {
      const r = await fetch("/api/vars", { method: "POST", headers: { "X-Requested-With": "sshpocket" }, body: JSON.stringify({ server: vSrv, package: vPkg, name: v.name, value, action, ack }) });
      return { ok: r.ok, status: r.status, body: await r.json().catch(() => ({})) };
    };
    const done = async text => {
      toast(text); resetCatalog(); // katalog kartlarındaki değerler yeniden okunsun
      window.KeyLabCompare.invalidate(); if (appState.view === "compare") window.KeyLabCompare.load(false);
      try { appState.srv = await json("/api/server?name=" + encodeURIComponent(vSrv)); if (appState.view === "server") renderServer(); } catch (e) { /* bir sonraki turda */ }
      const top = $("#vars-list").parentElement.scrollTop; await loadVars(); $("#vars-list").parentElement.scrollTop = top;
    };
    save.addEventListener("click", async () => {
      save.disabled = true;
      const r = await post("set", ctl.value);
      if (r.ok) return done(`${v.name} kaydedildi`);
      if (r.status === 409 && r.body.needAck) { ack = true; save.disabled = false; save.textContent = "Onayla ve kaydet"; return say("warn", "⚠ " + String(r.body.error).replace(/[.;\s]+$/, "") + ". Yine de kaydetmek için tekrar tıklayın."); }
      save.disabled = false; say("err", "✖ " + (r.body.error || "kaydedilemedi"));
    });
    const reset = $("[data-reset]", rowEl);
    if (reset) reset.addEventListener("click", async () => { reset.disabled = true; const r = await post("unset", ""); if (r.ok) return done(`${v.name} varsayılana döndü`); reset.disabled = false; say("err", "✖ " + (r.body.error || "sıfırlanamadı")); });
  }
  $("#vars-q").addEventListener("input", e => { vQ = e.target.value.trim(); paintVars(); });
  $("#vars-set").addEventListener("change", e => { vOnlySet = e.target.checked; paintVars(); });
  const closeVars = () => varsm.classList.remove("open");
  $("#vars-x").addEventListener("click", closeVars); $("#vars-close").addEventListener("click", closeVars);

  return { open: openVars, close: closeVars, cur: varCur };
}
