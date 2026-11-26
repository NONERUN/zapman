// The Status and journal dialog.
import { $, call, copyText, init } from "./common.js";

async function refresh() {
  $("txt").value = await call("StatusText");
}

$("btnCopy").addEventListener("click", () => copyText($("txt").value));
$("btnClose").addEventListener("click", () => call("CloseDialog", "status"));

init(refresh, ["main"]);
