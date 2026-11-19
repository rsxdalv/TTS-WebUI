# Browser checks (Playwright + Edge)

Not run by CI. `npm i playwright-core`, then `node full-load.js [headed]` with a Go proxy on the URL inside it
(claims `data-outputs` and `data-bark-voices` on, an output and a voice added after the build) loads each data page
fresh and reports the cookie, the `/api/go-data/*` requests and whether the new item is on screen.
`raf-gate.js` repeats the load with `requestAnimationFrame` normal, never firing and firing late: Next's pages router
hydrates (and so runs every `useEffect`) only when the tab produces frames. See R1F in the PR for the finding.

`static-export-check.js` and `staticsrv.js` check a static export (`npm run build:static`, CONTRACTS.md section 10): run
`node staticsrv.js <react-ui/out> 34010 <react-ui/public>`, put `gosrv -next 127.0.0.1:34010 -routes all` in front of it, then
`node static-export-check.js http://127.0.0.1:<gosrv port> <react-ui/out> <an extension name>`. It loads every page and
fails on hydration, console, request and 4xx/5xx problems, then follows sidebar links to confirm that no page reload happens.
`/api/gradio/*` errors need the Python backend and are ignored unless `-gradio` is given.
