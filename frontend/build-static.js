// `npm run build:static`: builds the React app with Vite into frontend/out.
// Go serves it (CONTRACTS.md section 10).
//
// The build goes to .out-building and is renamed to out/ only when it is
// complete, so a failed or interrupted build leaves the previous out/ as it
// was. out/_tts-export.json marks a complete export and records the commit it
// was built from, which Go compares with the install's HEAD to tell a stale
// export.

const fs = require("fs");
const path = require("path");
const { spawnSync } = require("child_process");

const root = __dirname;
const p = (...a) => path.join(root, ...a);
const BUILDING = p(".out-building");
const OUT = p("out");

const rm = (d) => fs.rmSync(d, { recursive: true, force: true, maxRetries: 5 });

function run(cmd, args) {
  const r = spawnSync(cmd, args, {
    cwd: root,
    stdio: "inherit",
    shell: process.platform === "win32",
    env: process.env,
  });
  return r.status === 0;
}

// sourceCommit is the git commit of the install the build comes from, or null
// outside a git checkout. Go compares it with HEAD to tell a stale build.
function sourceCommit() {
  const r = spawnSync("git", ["rev-parse", "HEAD"], {
    cwd: path.join(root, ".."),
    encoding: "utf8",
    shell: process.platform === "win32",
  });
  const out = (r.stdout || "").trim();
  return r.status === 0 && /^[0-9a-f]{40,64}$/.test(out) ? out : null;
}

function main() {
  // generates src/extensions/link.ts, which the app imports
  if (!run("node", ["generate-plugin-imports.js"])) return 1;

  rm(BUILDING);
  if (!run("npx", ["vite", "build", "--outDir", BUILDING, "--emptyOutDir"])) {
    rm(BUILDING);
    return 1;
  }

  fs.writeFileSync(
    path.join(BUILDING, "_tts-export.json"),
    JSON.stringify({ builtAt: new Date().toISOString(), commit: sourceCommit() }) + "\n"
  );
  // Vite copies public/ into the export. Uploads (file-input-cache) are served
  // live from public/ by Go, and a copy here would be stale.
  rm(path.join(BUILDING, "file-input-cache"));
  rm(OUT);
  fs.renameSync(BUILDING, OUT);
  console.log("build:static: export written to frontend/out");
  return 0;
}

if (require.main === module) process.exit(main());
