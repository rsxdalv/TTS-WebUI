# Browser checks (Playwright + Edge)

Not run by CI. `npm i playwright-core`, then `node full-load.js [headed]` with a Go proxy on the URL inside it
(claims `data-outputs` and `data-bark-voices` on, an output and a voice added after the build) loads each data page
fresh and reports the cookie, the `/api/go-data/*` requests and whether the new item is on screen.
`raf-gate.js` repeats the load with `requestAnimationFrame` normal, never firing and firing late: Next's pages router
hydrates (and so runs every `useEffect`) only when the tab produces frames. See R1F in the PR for the finding.
