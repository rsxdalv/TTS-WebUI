"use strict";
(function () {
  var kind = document.body.dataset.kind;
  var token = document.querySelector('meta[name="tts-ui-token"]').content;
  var app = document.getElementById("app");

  function el(tag, props, children) {
    var e = document.createElement(tag);
    Object.keys(props || {}).forEach(function (k) { e[k] = props[k]; });
    (children || []).forEach(function (c) {
      e.appendChild(typeof c === "string" ? document.createTextNode(c) : c);
    });
    return e;
  }

  function safeLink(url, text) {
    if (!/^https?:\/\//i.test(url || "")) return document.createTextNode(text);
    return el("a", { href: url, target: "_blank", rel: "noopener noreferrer", textContent: text });
  }

  function fail(message) {
    app.replaceChildren(el("div", { className: "notice", textContent: message }));
  }

  async function load() {
    var r = await fetch("api/list?kind=" + encodeURIComponent(kind), { credentials: "same-origin" });
    if (!r.ok) throw new Error("Could not load the extension list (" + r.status + ").");
    return r.json();
  }

  async function run(op, pkg, out, buttons) {
    out.textContent = "";
    buttons.forEach(function (b) { b.disabled = true; });
    try {
      var r = await fetch("api/op/" + op, {
        method: "POST",
        credentials: "same-origin",
        headers: { "Content-Type": "application/json", "X-TTS-UI": token },
        body: JSON.stringify({ package_name: pkg }),
      });
      if (!r.ok) {
        var msg = r.status;
        try { msg = (await r.json()).error || msg; } catch (e) { /* not JSON */ }
        out.textContent = "Failed: " + msg;
        return;
      }
      var reader = r.body.getReader();
      var decoder = new TextDecoder();
      var buf = "";
      for (;;) {
        var chunk = await reader.read();
        if (chunk.done) break;
        buf += decoder.decode(chunk.value, { stream: true });
        var lines = buf.split("\n");
        buf = lines.pop();
        lines.forEach(function (l) {
          if (!l) return;
          var m;
          try { m = JSON.parse(l); } catch (e) { return; }
          if (typeof m.line === "string") out.textContent += m.line + "\n";
          if (m.done) out.textContent += m.ok ? "Done.\n" : "Failed.\n";
        });
        out.scrollTop = out.scrollHeight;
      }
    } catch (e) {
      out.textContent += "Failed: " + e.message + "\n";
    } finally {
      buttons.forEach(function (b) { b.disabled = false; });
    }
  }

  function picker(label, names, opLabel, op, readOnly, out, all) {
    var select = el("select", { "aria-label": label });
    names.forEach(function (n) { select.appendChild(el("option", { value: n, textContent: n })); });
    var button = el("button", { textContent: opLabel, disabled: readOnly || names.length === 0 });
    button.addEventListener("click", function () {
      if (select.value) run(op, select.value, out, all());
    });
    return { box: el("div", { className: "col" }, [el("h2", { textContent: label }), select, button]), button: button };
  }

  function render(data) {
    var parts = [];
    if (data.warning) parts.push(el("div", { className: "notice", textContent: data.warning }));
    if (data.read_only) {
      parts.push(el("div", { className: "notice", textContent: "Installing and uninstalling are off while the app is open to other devices." }));
    }
    var rows = data.entries.map(function (e) {
      var name = el("td", {}, [safeLink(e.website, e.name || e.package_name)]);
      if (e.builtin) name.appendChild(el("span", { className: "tag", textContent: "built-in" }));
      if (e.disabled) name.appendChild(el("span", { className: "tag", textContent: "disabled" }));
      var state = e.installed ? (e.version ? "v" + e.version : "installed") : "not installed";
      return el("tr", {}, [name, el("td", { textContent: e.description || "" }), el("td", { className: "muted", textContent: state })]);
    });
    parts.push(el("table", {}, [
      el("thead", {}, [el("tr", {}, [el("th", { textContent: "Title" }), el("th", { textContent: "Description" }), el("th", { textContent: "State" })])]),
      el("tbody", {}, rows),
    ]));
    var out = el("pre", { "aria-live": "polite" });
    var buttons = [];
    var all = function () { return buttons; };
    var installs = picker("Install extension", data.choices, "Install extension", "install", data.read_only, out, all);
    var uninstalls = picker("Uninstall extension", data.uninstallable, "Uninstall extension", "uninstall", data.read_only, out, all);
    buttons.push(installs.button, uninstalls.button);
    parts.push(el("div", { className: "row" }, [installs.box, uninstalls.box]), out);
    app.replaceChildren.apply(app, parts);
  }

  load().then(render).catch(function (e) { fail(e.message); });
})();
