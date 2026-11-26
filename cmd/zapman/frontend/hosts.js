// The Compare hosts dialog.
import { $, call, copyText, init, t } from "./common.js";

let copied = false;

async function refresh() {
  $("lblCopied").textContent = copied ? t("HostsCopied") : "";
}

$("btnCopy").addEventListener("click", async () => {
  await copyText(await call("HostsView"));
  copied = true;
  refresh();
});
$("btnOpen").addEventListener("click", () => call("HostsOpen"));
$("btnClose").addEventListener("click", () => call("CloseDialog", "hosts"));

init(refresh);
