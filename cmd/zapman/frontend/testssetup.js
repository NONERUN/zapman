// The Run Tests dialog.
import { $, call, init, t } from "./common.js";

let built = false;

async function refresh() {
  const v = await call("SetupView");
  $("lblPick").textContent = t("TestsPick", v.Engine);
  if (built) {
    return;
  }
  // The list is made one time, so that the checks of the user stay when
  // the language changes.
  built = true;
  const checked = new Set(v.Checked ?? []);
  const list = $("list");
  list.replaceChildren();
  for (const name of v.Names ?? []) {
    const l = document.createElement("label");
    l.className = "check";
    const cb = document.createElement("input");
    cb.type = "checkbox";
    cb.value = name;
    cb.checked = checked.has(name);
    const s = document.createElement("span");
    s.textContent = name;
    l.append(cb, s);
    list.append(l);
  }
}

const boxes = () => [...$("list").querySelectorAll("input")];

$("btnAll").addEventListener("click", () => boxes().forEach((b) => (b.checked = true)));
$("btnNone").addEventListener("click", () => boxes().forEach((b) => (b.checked = false)));
$("btnRun").addEventListener("click", () => {
  const typ = $("rbDpi").checked ? "dpi" : "standard";
  call("TestsStart", typ, boxes().filter((b) => b.checked).map((b) => b.value));
});
$("btnCancel").addEventListener("click", () => call("CloseDialog", "testssetup"));

init(refresh);
