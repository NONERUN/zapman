// The Strategy dialog.
import { $, call, init } from "./common.js";

let engine = "";
let view = null;
// picked is the name of the selected item. It stays selected when the list
// is made again.
let picked = "";
let busy = false;

function selected() {
  return view?.Items.find((it) => it.Name === picked);
}

function update() {
  const it = selected();
  const ok = !busy && view?.EngineOK && it?.Supported;
  $("btnInst").disabled = !ok;
  $("btnRun").disabled = !ok;
  $("btnTests").disabled = busy;
  $("btnNone").disabled = busy;
  $("rbWinws").disabled = busy;
  $("rbWinws2").disabled = busy;
  for (const el of $("list").children) {
    el.classList.toggle("sel", el.dataset.name === picked);
  }
}

async function refresh() {
  const [v, m] = await Promise.all([call("StrategyView", engine), call("Main")]);
  view = v;
  busy = m.Busy;
  engine = v.Engine;
  $("rbWinws").checked = engine === "winws";
  $("rbWinws2").checked = engine === "winws2";
  if (!v.Items.some((it) => it.Name === picked)) {
    picked = v.Items[v.Selected]?.Name ?? "";
  }
  const list = $("list");
  list.replaceChildren();
  for (const it of v.Items) {
    const el = document.createElement("div");
    el.className = "item" + (it.Supported ? "" : " off");
    el.textContent = it.Label;
    el.dataset.name = it.Name;
    el.addEventListener("click", () => {
      picked = it.Name;
      update();
    });
    el.addEventListener("dblclick", () => {
      picked = it.Name;
      update();
      if (!$("btnRun").disabled) {
        $("btnRun").click();
      }
    });
    list.append(el);
  }
  update();
}

$("list").addEventListener("keydown", (e) => {
  if (!view || (e.key !== "ArrowDown" && e.key !== "ArrowUp")) {
    return;
  }
  e.preventDefault();
  const i = view.Items.findIndex((it) => it.Name === picked);
  const j = Math.max(0, Math.min(view.Items.length - 1, i + (e.key === "ArrowDown" ? 1 : -1)));
  picked = view.Items[j]?.Name ?? picked;
  update();
  $("list").children[j]?.scrollIntoView({ block: "nearest" });
});

for (const id of ["rbWinws", "rbWinws2"]) {
  $(id).addEventListener("change", (e) => {
    if (e.target.checked) {
      engine = e.target.value;
      refresh();
    }
  });
}
$("btnInst").addEventListener("click", () => call("StrategyInstall", picked, engine));
$("btnRun").addEventListener("click", () => call("StrategyRun", picked, engine));
$("btnTests").addEventListener("click", () => call("StrategyTests", engine));
$("btnNone").addEventListener("click", () => call("StrategyNone"));
$("btnCancel").addEventListener("click", () => call("CloseDialog", "strategy"));

init(refresh, ["strategy", "main"]);
