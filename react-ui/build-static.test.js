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

test("an interrupted export: .next is the scratch directory, the saved server build comes back", () => {
  const d = tmp();
  put(d, ".next-server-saved", { BUILD_ID: "old" });
  put(d, ".next", { "package.json": "scratch" });
  recover(d);
  expect(fs.readFileSync(path.join(d, ".next", "BUILD_ID"), "utf8")).toBe("old");
  expect(fs.existsSync(path.join(d, ".next-server-saved"))).toBe(false);
});

test("an interrupted export and no .next at all: the saved build comes back", () => {
  const d = tmp();
  put(d, ".next-server-saved", { BUILD_ID: "old" });
  recover(d);
  expect(fs.readFileSync(path.join(d, ".next", "BUILD_ID"), "utf8")).toBe("old");
});

test("a newer server build in .next is kept, the older saved one is dropped", () => {
  const d = tmp();
  put(d, ".next-server-saved", { BUILD_ID: "old" });
  put(d, ".next", { BUILD_ID: "new" });
  recover(d);
  expect(fs.readFileSync(path.join(d, ".next", "BUILD_ID"), "utf8")).toBe("new");
  expect(fs.existsSync(path.join(d, ".next-server-saved"))).toBe(false);
});

test("nothing saved: nothing changes", () => {
  const d = tmp();
  put(d, ".next", { BUILD_ID: "cur" });
  recover(d);
  expect(fs.readFileSync(path.join(d, ".next", "BUILD_ID"), "utf8")).toBe("cur");
});
