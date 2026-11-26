// The main window.
import { $, call, init } from "./common.js";

const plain = ["btnRemove", "btnFakes", "btnGames", "btnIpset", "btnHosts", "btnVersion", "btnDiag",
  "cmbGame", "cmbIpset", "chkAuto", "chkTray", "rbRu", "rbEn"];

async function refresh() {
  const v = await call("Main");
  document.title = v.Title;
  $("rbRu").checked = v.Lang === "ru";
  $("rbEn").checked = v.Lang !== "ru";

  const b = $("banner");
  b.textContent = v.Status.Banner.Text;
  b.style.background = v.Status.Banner.Bg;
  b.style.color = v.Status.Banner.Fg;

  $("lblBypass").textContent = v.Status.Bypass;
  $("lblService").textContent = v.Status.Service;
  $("lblInstalled").textContent = v.Status.Installed;
  $("lblDivert").textContent = v.Status.Divert;

  $("cmbGame").selectedIndex = v.GameFilter;
  $("cmbIpset").selectedIndex = v.IPSet;
  $("chkAuto").checked = v.AutoUpdate;
  $("chkTray").checked = v.TrayWatch;

  // As Set-GuiBusy and Update-Status: all controls are off while an action
  // runs; Start needs a service, Strategy... needs strategy files.
  for (const id of plain) {
    $(id).disabled = v.Busy;
  }
  $("btnStop").disabled = v.Busy;
  $("btnStart").disabled = v.Busy || !v.HasSvc;
  $("btnStrategy").disabled = v.Busy || !v.HasStrt;
  $("grpStatus").classList.toggle("off", v.Busy);
}

const click = (id, method) => $(id).addEventListener("click", () => call(method));
click("btnStart", "ClickStart");
click("btnStop", "ClickStop");
click("btnRemove", "ClickRemove");
click("btnStrategy", "ClickStrategy");
click("btnFakes", "ClickFakes");
click("btnGames", "ClickGames");
click("btnIpset", "ClickIPSet");
click("btnHosts", "ClickHosts");
click("btnVersion", "ClickVersion");
click("btnDiag", "ClickDiag");
click("grpStatus", "ClickStatus");

for (const id of ["rbRu", "rbEn"]) {
  $(id).addEventListener("change", (e) => {
    if (e.target.checked) {
      call("SetLanguage", e.target.value);
    }
  });
}
$("cmbGame").addEventListener("change", (e) => call("SetGameFilter", e.target.selectedIndex));
$("cmbIpset").addEventListener("change", (e) => call("SetIPSet", e.target.selectedIndex));
$("chkAuto").addEventListener("change", (e) => call("SetAutoUpdate", e.target.checked));
$("chkTray").addEventListener("change", (e) => call("SetTrayWatch", e.target.checked));

init(refresh, ["main"]);
