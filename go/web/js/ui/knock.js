/* KeyLab — port knocking formu (sunucu sayfası > Özet > Port knocking…). app.js'ten çıkarıldı (refaktör 2c), kod birebir.
   Dizi Keychain'dedir (bu bilgisayara özgü; USB başka bir Mac'e takılırsa orada yeniden girilmelidir). Arayüz diziyi ASLA geri okumaz.
   (app.js'teki hiçbir yerde çağrılmayan knockText yardımcısı ölü koddu; taşınmadı.) */
import { $, esc, json, toast } from "../lib/util.js";

export const knockSeqOK = v => /^[0-9]+(,[0-9]+){0,9}$/.test(v) && v.split(",").every(p => +p >= 1 && +p <= 65535);

// hooks: { openDlg, closeDlg } genel iletişim kutusu; { reloadServer } kayıttan sonra sunucu sayfasını yeniler
export function initKnock({ openDlg, closeDlg, reloadServer }) {
  async function openKnock(name) {
    let k;
    try { k = await json("/api/knock?name=" + encodeURIComponent(name)); }
    catch (e) { toast("Okunamadı: " + e.message); return; }
    const state = !k.enabled ? "" : k.hasSeq
      ? `<div class="notice ok sj-da12f2"><span class="ico" aria-hidden="true">✔</span><div>Knock <b>açık</b>: ${esc(k.proto)}, ${esc(k.delay)} ms. Dizi bu bilgisayarın Keychain'inde kayıtlı; güvenlik için tamamı gösterilmez${k.hint ? `, yalnızca her portun ilk ve son rakamı: <span class="mono sj-eed0f8">${esc(k.hint)}</span>` : ""}. Yeni bir dizi girerseniz eskisinin yerine geçer.</div></div>`
      : `<div class="notice warn sj-da12f2"><span class="ico" aria-hidden="true">⚠</span><div>Bu sunucuda knock <b>açık</b> görünüyor ama dizi <b>bu bilgisayarın</b> Keychain'inde yok. Dizi USB'ye değil, yazıldığı Mac'e kaydedilir; USB'yi başka bir Mac'e taktıysanız diziyi burada yeniden girin.</div></div>`;
    openDlg({
      title: "Port knocking", sub: name,
      body: `${state}<p class="small muted sj-249d01">SSH portu kapalıyken, sunucu yöneticisinin belirlediği port sırasına "vurarak" açtırma yöntemi. Sunucuda knock kurulumu yönetici tarafından yapılmış olmalıdır.</p>
        <div class="field"><label for="kn-seq">Dizi (port sırası)</label><input id="kn-seq" type="password" autocomplete="off" spellcheck="false" data-lpignore="true" data-1p-ignore="true" placeholder="${k.hasSeq ? "•••••• kayıtlı dizi korunur" : "ör. 7000,8000,9000"}" inputmode="numeric"><div class="hint" id="kn-hint">${k.hasSeq ? "Değiştirmek için yeni diziyi tam olarak yazın. Boş bırakırsanız kayıtlı dizi korunur ve yalnızca protokol ile bekleme güncellenir." : "Sunucu yöneticisinin verdiği PORT SIRASI: virgülle, boşluksuz, en çok 10 port. Adım numarasını değil, portları yazın."}</div>
</div>
        <div class="small sj-fe7b49"><label class="sj-8286c2"><input type="checkbox" id="kn-show"> Yazdıklarımı göster</label></div>
        <div class="row-flex sj-9d2d06">
          <div class="field sj-116966"><label for="kn-proto">Protokol</label><select class="sj-f4596f" id="kn-proto"><option value="tcp">tcp</option><option value="udp">udp</option></select></div>
          <div class="field sj-116966"><label for="kn-delay">Vuruşlar arası bekleme (ms)</label><input class="sj-607468" id="kn-delay" type="number" min="0" max="5000" step="50"></div></div>
        ${k.enabled ? `<div class="row-flex sj-bf0907">${k.hasSeq ? '<button class="btn sm" id="kn-send" type="button">Knock\'u şimdi gönder</button>' : ""}<button class="btn sm dangerbtn" id="kn-off" type="button">Knock\'u kapat ve kaydı sil</button></div>` : ""}`,
      ok: "Kaydet",
      onOk: async () => {
        const seq = $("#kn-seq").value.trim(), delay = $("#kn-delay").value.trim();
        if (seq ? !knockSeqOK(seq) : !k.hasSeq) { $("#dlg-err").textContent = seq ? "Dizi 1-10 port olmalı: virgülle ayrılmış, her biri 1-65535, boşluksuz (örnek: 7000,8000,9000)." : "Dizi gerekli: bu bilgisayarda kayıtlı dizi yok (örnek: 7000,8000,9000)."; return; }
        if (!/^[0-9]+$/.test(delay) || +delay > 5000) { $("#dlg-err").textContent = "Bekleme 0 ile 5000 arasında bir sayı olmalı."; return; }
        $("#dlg-ok").disabled = true;
        try { await json("/api/knock", { name, action: "save", proto: $("#kn-proto").value, delay, seq }); }
        catch (e) { $("#dlg-ok").disabled = false; $("#dlg-err").textContent = "✖ " + e.message; return; }
        $("#kn-seq").value = "";
        closeDlg(); toast("Knock kaydedildi"); await reloadServer();
      },
    });
    $("#kn-proto").value = k.proto || "tcp"; $("#kn-delay").value = k.delay || "300";
    const seq = $("#kn-seq"), hint = $("#kn-hint");
    seq.addEventListener("input", () => { const v = seq.value.trim(); hint.classList.toggle("err", !!v && !knockSeqOK(v)); $("#dlg-err").textContent = ""; });
    seq.addEventListener("keydown", e => { if (e.key === "Enter") $("#dlg-ok").click(); });
    $("#kn-show").addEventListener("change", e => { seq.type = e.target.checked ? "text" : "password"; });
    $("#kn-send")?.addEventListener("click", async () => {
      try { await json("/api/knock", { name, action: "send" }); toast("Knock gönderildi"); } catch (e) { $("#dlg-err").textContent = "✖ " + e.message; }
    });
    let sure = false;
    $("#kn-off")?.addEventListener("click", async e => {
      if (!sure) { sure = true; e.target.textContent = "Emin misiniz? Tekrar tıklayın"; return; }
      try { await json("/api/knock", { name, action: "disable" }); } catch (er) { $("#dlg-err").textContent = "✖ " + er.message; return; }
      closeDlg(); toast("Knock kapatıldı"); await reloadServer();
    });
    setTimeout(() => seq.focus(), 50);
  }
  return { open: openKnock };
}
