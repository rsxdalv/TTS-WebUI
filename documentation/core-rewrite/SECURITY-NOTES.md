# Security notes: same-origin trust, roles, untrusted content

Written 2026-11-28 while the launcher, the React frontend and the Gradio tabs
were being pulled closer together. These are things to keep in mind, not
decisions.

## What we accept

- An installed extension is trusted code: a Gradio extension runs arbitrary
  Python with the user's rights. Installing one is already remote code
  execution by design. The installer and the extension manager are therefore the
  most sensitive surface of the app.
- The frontend already calls Gradio (the wrapped tabs are iframes of it), and the
  plan is to move more into the frontend. Keeping the extension manager out of
  the frontend's *page* does not remove the link: once frontend and Gradio meet
  in one origin, a script in either can reach the other.

## Where things stand (one port)

Since the router serves the React UI at `/`, Gradio under `/gradio-legacy/`
and extension pages at `/<package>` on the public port (7770), they all share one
origin. A script in an extension's page or in the Gradio UI can reach the
frontend's storage and its API (and the other way round). The launcher (7700) is
still its own origin with its own token. The React UI's Host/Origin guards use
the public port; its LAN mode and allowed host names still apply (`TTS_GO_PROXY_LISTEN`,
`TTS_GO_PROXY_ALLOWED_HOSTS`). `TTS_FRONT_IN_ROUTER=0` restores the two-port layout.

## What does and does not protect

- A token held in page memory (an inline `<script>token=…</script>`, a meta tag)
  stops other origins, `curl`, other local processes and LAN peers. It does **not**
  stop script that runs in the same origin: it can read globals, re-import the
  API module, wrap `fetch`, fetch the page again and parse the token out, or just
  click the real buttons. Same-origin XSS has the full power of that origin.
- What limits same-origin XSS: a separate origin for anything with admin power, a
  strict CSP (no inline script; a token in an inline script then needs a nonce),
  untrusted content in sandboxed iframes, few dependencies in the code that holds
  admin power (the frontend has hundreds of npm packages that run same-origin: a
  compromised one counts as XSS).
- Login alone does not stop XSS either: a script in a logged-in admin's origin
  acts as the admin. Dangerous actions (install, uninstall, update, enabling an
  extension) want a step-up (password again, or a confirmation in a separate
  admin origin).

## Rules to follow when merging things

1. **Roles are enforced on the server, on every admin route.** The UI hiding a
   button is not a check. Add a test that a non-admin gets 403 on each admin
   route. (Open WebUI's gates were bypassed more than once by a route that
   missed the check: CVE-2026-45395 workspace tools, CVE-2026-45672 code execute.)
2. **Never give user-supplied content `allow-scripts` together with
   `allow-same-origin`.** That combination defeats the sandbox (Open WebUI
   CVE-2026-45303, CVE-2026-70486). Chain previews, outputs, extension-rendered
   HTML and anything a model or a dropped workflow file produces go in a sandboxed
   iframe with an opaque origin.
3. **Treat dropped chain/workflow files as untrusted data**, not as code or
   markup.
4. **Binding follows exposure.** The launcher stays loopback-only. Turning on LAN
   mode for the app must not expose install/update/uninstall; with roles, LAN
   mode requires an admin password.
5. **Separate origins are cheap on one port only for loopback names**
   (`127.0.0.1` vs `localhost` are different origins). On a LAN, hosts are
   usually bare IPs, so the split would need another port or real host names.
6. The unauthenticated `/api/health` may carry a per-start id; it must never
   carry or be derived from the token.

## Open questions

- One port and one UI: identity (roles) becomes the boundary instead of the port.
  See the discussion of Host-based routing versus path prefixes in the session
  notes; neither is built.
- Whether the admin UI should live in a separate origin even if it looks like one
  UI (embedded, with a narrow `postMessage` interface).
