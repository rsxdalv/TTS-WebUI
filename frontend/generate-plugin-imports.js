const fs = require("fs");

const getPackageJSON = () => {
  try {
    return require("./src/extensions/package.json").dependencies;
  } catch (error) {
    return {};
  }
};

const main = async () => {
  const installedPackages = getPackageJSON();

  if (!installedPackages || Object.keys(installedPackages).length === 0) {
    console.log("No React UI extensions found");

    fs.writeFileSync(
      "./src/extensions/link.ts",
      `const linkExtension = {};
export default linkExtension;
`
    );
    return;
  }

  const ttsWebuiPackages = Object.keys(installedPackages).filter((x) =>
    x.startsWith("@tts-webui")
  );

  console.log("Found React UI extensions:", ttsWebuiPackages.join(", "));

  const imports = ttsWebuiPackages
    .map(
      (x) =>
        `export { default as ${x.replace("@tts-webui/", "")} } from "${x}";`
    )
    .join("\n");

  fs.writeFileSync("./src/extensions/link.ts", imports);

  console.log("Generated React UI extensions imports");

  return;
};

main();
