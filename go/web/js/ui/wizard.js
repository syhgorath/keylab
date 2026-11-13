/* KeyLab — sunucu "Sihirbaz" penceresi (bağlam menüsü > Taslak sihirbaz; TASLAK: komut çalıştırmaz). app.js'ten çıkarıldı (refaktör 2c),
   kod birebir; tek fark: home artık hooks.getHome() ile okunur (home yeniden atanabilen durumdur, doğrudan paylaşılamaz). */
import { $, $$ } from "../lib/util.js";

// hooks: { getHome() } güncel {servers, packages}
export function initWizard({ getHome }) {
  const wiz = $("#wizard"), confirmInput = $("#confirm-input"), nextBtn = $("#next-btn"), applyBtn = $("#apply-btn"), finishBtn = $("#finish-btn");
  let wname = "";
  function setStep(n) {
    [1, 2, 3].forEach(i => ($("#step-" + i).hidden = i !== n));
    $$("#wiz-steps .step").forEach((t, i) => { t.classList.toggle("active", i + 1 === n); t.classList.toggle("done", i + 1 < n); });
    nextBtn.hidden = n !== 1; applyBtn.hidden = n !== 2; finishBtn.hidden = n !== 3;
    if (n === 2) { confirmInput.value = ""; applyBtn.disabled = true; setTimeout(() => confirmInput.focus(), 40); }
  }
  function openWizard(name) {
    wname = name;
    const home = getHome();
    const s = home.servers.find(x => x.name === name), pkg = (s && s.last && s.last.package) || (home.packages[0] && home.packages[0].id) || "?";
    const ver = (home.packages.find(p => p.id === pkg) || {}).version || "?";
    $("#wiz-desc").textContent = `Paket: ${pkg} · Sunucu: ${name}`;
    $("#w-pkg").textContent = `ad:     ${pkg}\nsürüm:  ${ver}\nkaynak: USB`;
    $("#w-srv").textContent = `ad:     ${name}\ndurum:  ${s && s.up ? "çevrimiçi" : "bağlantı yok"}`;
    $("#wiz-confirm-name").textContent = name; confirmInput.placeholder = name;
    setStep(1); wiz.classList.add("open"); setTimeout(() => nextBtn.focus(), 40);
  }
  function closeWizard() { wiz.classList.remove("open"); }
  $$("[data-close]").forEach(b => b.addEventListener("click", closeWizard));
  nextBtn.addEventListener("click", () => setStep(2));
  confirmInput.addEventListener("input", () => { applyBtn.disabled = confirmInput.value.trim() !== wname; });
  applyBtn.addEventListener("click", () => { if (confirmInput.value.trim() === wname) setStep(3); });
  return { open: openWizard, close: closeWizard, isOpen: () => wiz.classList.contains("open") };
}
