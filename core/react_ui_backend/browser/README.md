# Browser check (Playwright + Edge)

Not run by CI. `npm i playwright-core`, then serve the install root's export with the Go server and point the check at it:

```
go run ./react_ui_backend/cmd/gosrv -root <install root> -port 34012
node react_ui_backend/browser/static-export-check.js http://127.0.0.1:34012 <react-ui/src/routes.tsx> <an extension name>
```

`static-export-check.js` loads every page of the export (`npm run build:static`, CONTRACTS.md section 10) and fails on
hydration, console, request and 4xx/5xx problems, then follows sidebar links to confirm that no page reload happens.
`/api/gradio/*` errors need the Python backend and are ignored unless `-gradio` is given.
