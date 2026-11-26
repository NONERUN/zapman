// The tests window.
import { $, call, init, t, table } from "./common.js";

// tab is the name of the selected tab. When the user did not select a tab,
// the last tab is selected, as the PS GUI does with each new strategy.
let tab = "";
let userTab = false;
let shown = -1;

function showTabs(tabs) {
  if (!userTab || !tabs.some((x) => x.Name === tab)) {
    tab = tabs.length > 0 ? tabs[tabs.length - 1].Name : "";
  }
  const list = $("tabList");
  if (shown !== tabs.length) {
    shown = tabs.length;
    list.replaceChildren();
    for (const x of tabs) {
      const el = document.createElement("div");
      el.className = "item" + (x.HasError ? " bad" : "");
      el.textContent = x.Name;
      el.dataset.name = x.Name;
      el.addEventListener("click", () => {
        tab = x.Name;
        userTab = true;
        showPane(tabs);
      });
      list.append(el);
    }
  }
  showPane(tabs);
}

function showPane(tabs) {
  for (const el of $("tabList").children) {
    el.classList.toggle("sel", el.dataset.name === tab);
  }
  const x = tabs.find((it) => it.Name === tab);
  table($("pane"), x ? x.Table : {});
}

async function refresh() {
  const v = await call("TestsView");
  $("lblStatus").textContent = v.Status;
  const bar = $("bar");
  if (v.Max > 0) {
    bar.max = v.Max;
    bar.value = v.Value;
  } else {
    bar.removeAttribute("value");
  }
  const head = $("txtHead");
  const end = head.scrollTop + head.clientHeight >= head.scrollHeight - 4;
  head.textContent = v.Head;
  if (end) {
    head.scrollTop = head.scrollHeight;
  }
  const tabs = v.Tabs ?? [];
  $("tabs").style.display = v.Wait ? "none" : "flex";
  $("lblWait").style.display = v.Wait ? "flex" : "none";
  showTabs(tabs);
  table($("sum"), v.Summary);
  $("lblBest").textContent = v.Best;
  const btn = $("btnStop");
  btn.textContent = t(v.Done ? "BtnClose" : "BtnCancel");
  btn.toggleAttribute("data-default", v.Done);
}

$("btnStop").addEventListener("click", () => call("CloseDialog", "tests"));

init(refresh, ["tests"]);
