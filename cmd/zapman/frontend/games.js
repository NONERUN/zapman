// The Games dialog.
import { $, call, init } from "./common.js";

let built = false;

async function refresh() {
  if (built) {
    return;
  }
  built = true;
  const items = await call("GamesView");
  const list = $("list");
  list.replaceChildren();
  for (const it of items ?? []) {
    const l = document.createElement("label");
    l.className = "check";
    const cb = document.createElement("input");
    cb.type = "checkbox";
    cb.value = it.ID;
    cb.checked = it.Checked;
    const s = document.createElement("span");
    s.textContent = it.Title;
    if (it.About) {
      // The text about the game goes on a second line, under the title.
      l.classList.add("about");
      const a = document.createElement("small");
      a.textContent = it.About;
      s.append(a);
    }
    l.append(cb, s);
    list.append(l);
  }
}

$("btnOk").addEventListener("click", () => {
  const ids = [...$("list").querySelectorAll("input")].filter((b) => b.checked).map((b) => b.value);
  call("GamesApply", ids);
});
$("btnCancel").addEventListener("click", () => call("CloseDialog", "games"));

init(refresh);
