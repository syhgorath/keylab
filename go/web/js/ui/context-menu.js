/* KeyLab — sunucu bağlam menüsü (sağ tık / F10 / ⋯ düğmesi). app.js'ten çıkarıldı (refaktör 2b), kod birebir.
   Menünün açılışı, konumu, odak/ok tuşu gezintisi ve dışarı tıklayınca kapanışı burada; seçilen eylemin ne yapacağı app.js'tedir (onPick). */
import { $, $$ } from "../lib/util.js";

// hooks: { onPick(eylem, sunucuAdı) } menü kapandıktan sonra çağrılır
export function initContextMenu({ onPick }) {
  const ctx = $("#ctx"); let ctxName = "";
  function openCtx(x, y, name) {
    ctxName = name; $("#ctx-head").textContent = name;
    ctx.classList.add("open");
    const r = ctx.getBoundingClientRect();
    ctx.style.left = Math.max(8, Math.min(x, innerWidth - r.width - 8)) + "px";
    ctx.style.top = Math.max(8, Math.min(y, innerHeight - r.height - 8)) + "px";
    const f = ctx.querySelector("button"); if (f) f.focus();
  }
  const closeCtx = () => ctx.classList.remove("open");
  document.addEventListener("click", e => { if (!ctx.contains(e.target)) closeCtx(); });
  ctx.addEventListener("keydown", e => {
    const items = $$("button", ctx), i = items.indexOf(document.activeElement);
    if (e.key === "ArrowDown") { e.preventDefault(); items[(i + 1) % items.length].focus(); }
    if (e.key === "ArrowUp") { e.preventDefault(); items[(i - 1 + items.length) % items.length].focus(); }
  });
  $$("#ctx button").forEach(b => b.addEventListener("click", () => { const a = b.dataset.ctx, name = ctxName; closeCtx(); onPick(a, name); }));
  return { open: openCtx, close: closeCtx };
}
