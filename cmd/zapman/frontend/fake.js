// The Fakes dialog.
import { $, call, init } from "./common.js";

function fill(sel, items, idx) {
  // Keep the choice of the user when the list is made again.
  const keep = sel.options.length > 0 ? sel.selectedIndex : idx;
  sel.replaceChildren();
  for (const it of items ?? []) {
    const o = document.createElement("option");
    o.textContent = it;
    sel.append(o);
  }
  sel.selectedIndex = keep;
}

async function refresh() {
  const v = await call("FakesView");
  fill($("cmbDiscord"), v.Discord, v.DiscordSelected);
  fill($("cmbGame"), v.Game, v.GameSelected);
}

$("btnOk").addEventListener("click", () => call("FakesApply", $("cmbDiscord").selectedIndex, $("cmbGame").selectedIndex));
$("btnCancel").addEventListener("click", () => call("CloseDialog", "fake"));

init(refresh);
