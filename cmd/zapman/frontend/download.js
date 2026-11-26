// The download window.
import { $, call, init } from "./common.js";

async function refresh() {
  const v = await call("DownloadView");
  $("lblStatus").textContent = v.Status;
  const bar = $("bar");
  if (v.Percent < 0) {
    bar.removeAttribute("value");
  } else {
    bar.value = v.Percent;
  }
}

$("btnCancel").addEventListener("click", () => call("DownloadCancel"));

init(refresh, ["download"]);
