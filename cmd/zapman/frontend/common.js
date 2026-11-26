// Common code of the pages. The Wails asset server gives the runtime at
// /wails/runtime.js; the pages import it as an ES module.
import { Call, Events, Clipboard } from "/wails/runtime.js";

const FQN = "github.com/NONERUN/zapman/internal/gui.App.";

// call calls a bound method of gui.App and gives a promise.
export function call(method, ...args) {
  return Call.ByName(FQN + method, ...args);
}

// copyText puts the text on the clipboard.
export function copyText(text) {
  return Clipboard.SetText(text);
}

export const $ = (id) => document.getElementById(id);

let strings = {};

// t gives the UI string of the key and puts the args into {0}, {1} ...
export function t(key, ...args) {
  const s = strings[key] ?? key;
  return s.replace(/\{(\d+)\}/g, (m, n) => (n < args.length ? String(args[n]) : m));
}

// applyStrings sets the text of the elements with data-t and the tooltip
// of the elements with data-tip.
function applyStrings(root) {
  for (const el of root.querySelectorAll("[data-t]")) {
    el.textContent = t(el.dataset.t);
  }
  for (const el of root.querySelectorAll("[data-tip]")) {
    el.title = t(el.dataset.tip);
  }
}

// serial gives a function that runs fn, one call at a time. A call while
// fn runs makes one more run after it.
function serial(fn) {
  let running = false;
  let again = false;
  const run = async () => {
    if (running) {
      again = true;
      return;
    }
    running = true;
    try {
      do {
        again = false;
        try {
          await fn();
        } catch (e) {
          console.error(e);
        }
      } while (again);
    } finally {
      running = false;
    }
  };
  return run;
}

// init reads the strings and the view, and reads them again on the events.
// The "lang" event always reads the strings and the view again.
export async function init(refresh, events = []) {
  const r = serial(refresh);
  const lang = serial(async () => {
    strings = await call("Strings");
    applyStrings(document);
    await refresh();
  });
  Events.On("lang", lang);
  for (const ev of events) {
    Events.On(ev, r);
  }
  document.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && e.target.tagName !== "TEXTAREA") {
      const b = document.querySelector("[data-default]");
      if (b && !b.disabled) {
        e.preventDefault();
        b.click();
      }
    } else if (e.key === "Escape") {
      const b = document.querySelector("[data-cancel]");
      if (b && !b.disabled) {
        e.preventDefault();
        b.click();
      }
    }
  });
  await lang();
}

// table fills the element with a test table: {Headers, Rows, Error}.
export function table(el, tb) {
  el.replaceChildren();
  if (tb.Error) {
    const p = document.createElement("div");
    p.className = "cell-err tbl-error";
    p.textContent = tb.Error;
    el.append(p);
    return;
  }
  if (!tb.Headers || tb.Headers.length === 0) {
    return;
  }
  const tbl = document.createElement("table");
  const head = tbl.createTHead().insertRow();
  for (const h of tb.Headers) {
    const th = document.createElement("th");
    th.textContent = h;
    head.append(th);
  }
  const body = tbl.createTBody();
  for (const row of tb.Rows ?? []) {
    const tr = body.insertRow();
    for (const c of row) {
      const td = tr.insertCell();
      td.textContent = c.Text;
      td.className = "cell-" + c.Kind;
    }
  }
  el.append(tbl);
}
