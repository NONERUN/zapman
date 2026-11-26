// The Games dialog.
import { $, call, init, t } from "./common.js";

let built = false;

// checkItem gives a label with a check box and a title. The about text
// goes on a second line, under the title.
function checkItem(value, title, about, checked) {
  const l = document.createElement("label");
  l.className = "check";
  const cb = document.createElement("input");
  cb.type = "checkbox";
  cb.value = value;
  cb.checked = checked;
  const s = document.createElement("span");
  s.textContent = title;
  if (about) {
    l.classList.add("about");
    const a = document.createElement("small");
    a.textContent = about;
    s.append(a);
  }
  l.append(cb, s);
  return { l, cb };
}

async function refresh() {
  if (built) {
    return;
  }
  built = true;
  const items = await call("GamesView");
  const list = $("list");
  list.replaceChildren();
  for (const it of items ?? []) {
    const box = document.createElement("div");
    box.className = "game";
    box.dataset.id = it.ID;
    const row = document.createElement("div");
    row.className = "game-row";
    const game = checkItem(it.ID, it.Title, it.About, it.Checked);
    game.cb.classList.add("game-on");
    row.append(game.l);
    if (it.HasLists) {
      const b = document.createElement("button");
      b.textContent = t("BtnGameLists");
      b.title = t("TipGameLists");
      b.addEventListener("click", () => call("GamesOpenLists", it.ID));
      row.append(b);
    }
    box.append(row);
    // The options of a game that is off are disabled; they keep the value.
    const opts = [];
    for (const o of it.Options ?? []) {
      const opt = checkItem(o.ID, o.Title, o.About, o.Checked);
      opt.l.classList.add("option");
      opt.cb.classList.add("game-option");
      opts.push(opt);
      box.append(opt.l);
    }
    const sync = () => {
      for (const o of opts) {
        o.cb.disabled = !game.cb.checked;
        o.l.classList.toggle("disabled", !game.cb.checked);
      }
    };
    game.cb.addEventListener("change", sync);
    sync();
    list.append(box);
  }
}

$("btnOk").addEventListener("click", () => {
  const ids = [];
  const opts = {};
  for (const box of $("list").querySelectorAll(".game")) {
    if (box.querySelector("input.game-on").checked) {
      ids.push(box.dataset.id);
    }
    const vals = {};
    for (const cb of box.querySelectorAll("input.game-option")) {
      vals[cb.value] = cb.checked;
    }
    opts[box.dataset.id] = vals;
  }
  call("GamesApply", ids, opts);
});
$("btnCancel").addEventListener("click", () => call("CloseDialog", "games"));

init(refresh);
