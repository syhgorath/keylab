/* KeyLab — sunucu yeniden adlandırma ve silme pencereleri (silme: onay → /api/server-remove → "Sunucu siliniyor" iş penceresi).
   app.js'ten çıkarıldı (refaktör 5e), kod BİREBİR. Testler: go/webtest/ops.test.js, runmodal2.test.js */
import { $, esc, json, toast } from "../lib/util.js";

// hooks: { openDlg, closeDlg, jobs (openRun, jobBusy, watchJob, setMode), load(), openServer(ad) }
//   →  { openRename(ad), openDelete(ad), getRemoval() → {name, purge} (iş penceresi bitişinde günlük silme seçimi) }
export function initServerOps(hooks) {
  const { openDlg, closeDlg, jobs, load, openServer } = hooks;
  const { openRun, jobBusy, watchJob } = jobs;
  let delName = "", delPurge = false;

  function openRename(name) {
    openDlg({
      title: "Sunucuyu yeniden adlandır", sub: name,
      body: `<div class="field"><label for="rn-new">Yeni ad</label><input id="rn-new" autocomplete="off" spellcheck="false" autocapitalize="off" maxlength="40" value="${esc(name)}"><div class="hint" id="rn-hint">Küçük harfle başlar; yalnızca küçük harf, rakam ve _ .</div></div>
        <ul class="small muted sj-dc5781"><li>Klasör ve anahtar dosyaları (<span class="mono">id_ed25519_${esc(name)}</span>) yeni ada taşınır; değişkenler ve sunucudaki kayıt geçmişi korunur.</li><li>Günlük ve sonuç geçmişi de yeni ada taşınır.</li><li><b>macOS Keychain</b>'deki passphrase ve Knock kaydı otomatik taşınmaz; passphrase kullanıyorsanız yeniden eklemeniz gerekir.</li></ul>`,
      ok: "Yeniden adlandır",
      onOk: async () => {
        const nn = $("#rn-new").value.trim();
        if (!/^[a-z][a-z0-9_]*$/.test(nn) || nn.length > 40) { $("#dlg-err").textContent = "Geçersiz ad."; return; }
        if (nn === name) { $("#dlg-err").textContent = "Yeni ad eskisiyle aynı."; return; }
        $("#dlg-ok").disabled = true;
        try { await json("/api/server-rename", { old: name, new: nn }); }
        catch (e) { $("#dlg-ok").disabled = false; $("#dlg-err").textContent = "✖ " + e.message; return; }
        closeDlg(); toast(`${name} → ${nn}`); await load(); openServer(nn);
      },
    });
    setTimeout(() => { const i = $("#rn-new"); i.focus(); i.select(); i.addEventListener("keydown", e => { if (e.key === "Enter") $("#dlg-ok").click(); }); }, 50);
  }

  async function openDelete(name) {
    if (jobBusy()) return;
    let info;
    try { info = await json("/api/server-files?name=" + encodeURIComponent(name)); }
    catch (e) { toast("Okunamadı: " + e.message); return; }
    const files = info.files || [], hasKey = files.some(f => f.isKey), state = info.state || [];
    openDlg({
      title: "Sunucuyu sil", sub: name,
      body: `<div class="notice danger"><span class="ico" aria-hidden="true">⚠</span><div><b>Bu işlem geri alınamaz.</b> <span class="mono">servers/${esc(name)}/</span> klasörü ve içindeki her şey silinir${hasKey ? "; <b>özel SSH anahtarı dahil</b>" : ""}. Sunucunun kendisine dokunulmaz.</div></div>
        <div class="small muted sj-2173cd">Silinecek dosyalar</div>
        <div class="chips">${files.map(f => `<span class="chip${f.isKey ? " chip-key" : ""}">${esc(f.name)}</span>`).join("") || '<span class="small muted">boş klasör</span>'}</div>
        ${hasKey ? '<label class="small sj-6693ed"><input type="checkbox" id="dl-key"> <span><b>Özel anahtarın silineceğini anlıyorum</b>; sunucuya giriş için tek yol buysa erişimi kaybedebilirim.</span></label>' : ""}
        ${state.length ? `<label class="small sj-6abd0a"><input type="checkbox" id="dl-logs"> <span>Bu bilgisayardaki günlük ve sonuç geçmişini de sil (${state.map(esc).join(", ")}). İşaretlemezseniz günlükler kalır.</span></label>` : ""}
        <div class="field sj-d6f2af"><label for="dl-name">Onay için sunucu adını yazın: <span class="mono">${esc(name)}</span></label><input id="dl-name" autocomplete="off" spellcheck="false" placeholder="${esc(name)}"><div class="hint">Büyük/küçük harf duyarlıdır. Motor Keychain kaydını silmek isteyip istemediğinizi ayrıca sorabilir.</div></div>`,
      ok: "Sil", okClass: "dangerbtn",
      onOk: async () => {
        $("#dlg-ok").disabled = true;
        let r;
        try { r = await json("/api/server-remove", { name, confirm: $("#dl-name").value, ack: !!($("#dl-key") && $("#dl-key").checked) }); }
        catch (e) { $("#dlg-ok").disabled = false; $("#dlg-err").textContent = "✖ " + e.message; return; }
        delName = name; delPurge = !!($("#dl-logs") && $("#dl-logs").checked);
        closeDlg();
        openRun("", "", "remove"); jobs.setMode("remove");
        $("#run-title").innerHTML = `Sunucu siliniyor <span class="draft-tag sj-0996f7">${esc(name)}</span>`;
        $("#run-sub").textContent = "KeyLab motoru (ModuleManager.sh remove)";
        watchJob(r.id);
      },
    });
    const check = () => { const okc = $("#dl-name").value === name && (!$("#dl-key") || $("#dl-key").checked); $("#dlg-ok").disabled = !okc; };
    $("#dlg-ok").disabled = true;
    ["#dl-name", "#dl-key"].forEach(id => { const e = $(id); if (e) { e.addEventListener("input", check); e.addEventListener("change", check); } });
    setTimeout(() => $("#dl-name").focus(), 50);
  }

  return { openRename, openDelete, getRemoval: () => ({ name: delName, purge: delPurge }) };
}
