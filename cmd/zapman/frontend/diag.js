// The Diagnostics dialog.
import { $, call, init, t } from "./common.js";

const marks = { ok: "✓", warn: "!", fail: "✗" };

async function refresh() {
  const v = await call("DiagView");
  const rows = $("rows");
  const top = rows.scrollTop;
  rows.replaceChildren();
  (v.Rows ?? []).forEach((r, i) => {
    const row = document.createElement("div");
    row.className = "row";
    const text = document.createElement("div");
    text.className = "text";
    text.textContent = r.Text;
    text.title = r.Text;
    const btn = document.createElement("button");
    btn.textContent = t("DiagRun");
    btn.disabled = v.Busy;
    btn.style.visibility = r.HasAction ? "visible" : "hidden";
    btn.addEventListener("click", () => call("DiagRun", i));
    const mark = document.createElement("div");
    mark.className = "mark " + r.Status;
    mark.textContent = marks[r.Status] ?? "";
    row.append(text, btn, mark);
    rows.append(row);
  });
  rows.scrollTop = top;
  $("btnOk").disabled = v.Busy;
}

$("btnOk").addEventListener("click", () => call("CloseDialog", "diag"));

init(refresh, ["diag"]);
