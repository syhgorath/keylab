/* KeyLab — karşılama (onboarding) kartı: hiç sunucu kayıtlı değilken Pano'nun üstünde 3 adımlı rehber gösterir.
   pocketui'deki karşılama kartının karşılığı; adımlar KeyLab'ın gerçek akışına bağlıdır (Sunucu ekle sihirbazı, ssh config içe aktarma,
   envanter yedeğinden geri yükleme). Salt görüntü: kendi başına bir şey yazmaz, yalnız ilgili sayfaları açar. */
(function () {
  "use strict";
  const $ = (s, r = document) => r.querySelector(s);
  const { html, setHtml } = window.KeyLabHtml;
  const isEn = () => window.KeyLabI18n && window.KeyLabI18n.getLang() === "en";
  const L = (tr, en) => (isEn() ? en : tr);
  let hooks = null, shown = null;

  function paint(empty) {
    const box = $("#onboarding");
    box.hidden = !empty;
    if (!empty) { shown = false; box.replaceChildren(); return; }
    shown = true;
    setHtml(box, html`
      <div class="ob-ico" aria-hidden="true">🚀</div>
      <div class="ob-body">
        <h2>${L("KeyLab'a hoş geldiniz", "Welcome to KeyLab")}</h2>
        <p class="muted small">${L("İlk sunucunuzu ekleyip SSH hazırlığını tamamlayın: 3 adımda filonuzu yönetmeye başlayın.", "Add your first server and finish the SSH setup: start managing your fleet in 3 steps.")}</p>
        <ol class="ob-steps">
          <li><b>${L("1. Sunucu ekle", "1. Add a server")}</b><span class="muted">${L("IP + kullanıcı ile server.env oluşturulur", "server.env is created from IP + user")}</span></li>
          <li><b>${L("2. SSH anahtarı", "2. SSH key")}</b><span class="muted">${L("ed25519 anahtar üret, ssh-agent'a ekle", "Generate an ed25519 key, add it to ssh-agent")}</span></li>
          <li><b>${L("3. İlk eylem", "3. First action")}</b><span class="muted">${L("Durumu oku, Lynis ile skorunuzu ölçün", "Read the status, measure your score with Lynis")}</span></li>
        </ol>
      </div>
      <div class="ob-actions">
        <button class="btn primary" id="ob-add" type="button">${L("Sunucu ekle →", "Add a server →")}</button>
        <button class="btn sm" id="ob-import" type="button">${L("~/.ssh/config'ten içe aktar", "Import from ~/.ssh/config")}</button>
        <button class="btn sm ghost" id="ob-restore" type="button">${L("Yedekten geri yükle", "Restore from backup")}</button>
      </div>`);
    $("#ob-add").addEventListener("click", () => hooks.goAdd());
    $("#ob-import").addEventListener("click", () => hooks.openImport());
    $("#ob-restore").addEventListener("click", () => hooks.goSettings());
  }

  // her çizimde çağrılır; yalnız durum (boş/dolu) ya da dil değişince yeniden çizer
  function render(empty) { if (empty !== shown || empty) paint(empty); }

  function init(h) { hooks = h; window.addEventListener("keylab:langchange", () => { if (shown) paint(true); }); }
  window.KeyLabOnboarding = { init, render };
})();
