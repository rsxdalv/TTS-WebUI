/** @jest-environment node */
const fs = require("fs");
const os = require("os");
const path = require("path");
const { recover } = require("./build-static");

const tmp = () => fs.mkdtempSync(path.join(os.tmpdir(), "bs-"));
const put = (dir, name, files) => {
  fs.mkdirSync(path.join(dir, name), { recursive: true });
  for (const [f, c] of Object.entries(files)) fs.writeFileSync(path.join(dir, name, f), c);
};

const mark = (d) => fs.writeFileSync(path.join(d, ".next-export-scratch"), "x");
const id = (d) => fs.readFileSync(path.join(d, ".next", "BUILD_ID"), "utf8");
const clean = (d) =>
  !fs.existsSync(path.join(d, ".next-server-saved")) && !fs.existsSync(path.join(d, ".next-export-scratch"));

test("1. killed before Next wrote BUILD_ID: the saved build comes back", () => {
  const d = tmp();
  put(d, ".next-server-saved", { BUILD_ID: "old" });
  put(d, ".next", { "package.json": "scratch" });
  mark(d);
  recover(d);
  expect(id(d)).toBe("old");
  expect(clean(d)).toBe(true);
});

test("1b. killed right after the rename: no .next at all", () => {
  const d = tmp();
  put(d, ".next-server-saved", { BUILD_ID: "old" });
  mark(d);
  recover(d);
  expect(id(d)).toBe("old");
  expect(clean(d)).toBe(true);
});

test("2. killed after the scratch has a BUILD_ID: the saved build still comes back", () => {
  const d = tmp();
  put(d, ".next-server-saved", { BUILD_ID: "old" });
  put(d, ".next", { BUILD_ID: "scratch", "export-detail.json": "{}" });
  mark(d);
  recover(d);
  expect(id(d)).toBe("old");
  expect(fs.existsSync(path.join(d, ".next", "export-detail.json"))).toBe(false);
  expect(clean(d)).toBe(true);
});

test("3. killed, then npm run build (prebuild recovers first, the build overwrites): the new build is kept", () => {
  const d = tmp();
  put(d, ".next-server-saved", { BUILD_ID: "old" });
  put(d, ".next", { BUILD_ID: "scratch" });
  mark(d);
  recover(d); // prebuild
  fs.writeFileSync(path.join(d, ".next", "BUILD_ID"), "new"); // next build
  recover(d); // a later build:static or S2 check finds nothing to do
  expect(id(d)).toBe("new");
  expect(clean(d)).toBe(true);
});

test("killed between the restore and the marker removal: the marker is removed, .next stays", () => {
  const d = tmp();
  put(d, ".next", { BUILD_ID: "old" });
  mark(d);
  recover(d);
  expect(id(d)).toBe("old");
  expect(clean(d)).toBe(true);
});

test("a saved copy without a marker is an orphan and is deleted", () => {
  const d = tmp();
  put(d, ".next-server-saved", { BUILD_ID: "old" });
  put(d, ".next", { BUILD_ID: "cur" });
  recover(d);
  expect(id(d)).toBe("cur");
  expect(clean(d)).toBe(true);
});

test("nothing saved: nothing changes", () => {
  const d = tmp();
  put(d, ".next", { BUILD_ID: "cur" });
  recover(d);
  expect(id(d)).toBe("cur");
});
