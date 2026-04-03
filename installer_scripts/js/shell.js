const { spawn, execSync, exec } = require("child_process");

const runCommand = (command, args) =>
  new Promise((resolve, reject) => {
    const child = spawn(command, args, { stdio: "inherit" });
    child.on("error", (error) => reject(new Error(`Error: ${error.message}`)));
    child.on("exit", (code) =>
      code === 0
        ? resolve()
        : reject(new Error(`Command failed with exit code ${code}`))
        // kill installer on error?
        // process.exit(code)
    );
  });

const $ = (command) =>
  console.log(`>${command}`) ||
  runCommand(command.split(" ")[0], command.split(" ").slice(1));

exports.$ = $;

const $$ = (command) =>
  console.log(`>${command.join(" ")}`) ||
  runCommand(command[0], command.slice(1));

exports.$$ = $$;

function $shSync(cmd) {
  try {
    console.log(`>${cmd}`);
    return execSync(cmd, { stdio: "inherit" });
  } catch (error) {
    try {
      console.error(error.stderr.toString().trim(), error.status);
    } catch (e) {
      console.error(error);
    }
    throw error;
  }
}

exports.$shSync = $shSync;

// Non-blocking version of $sh that uses spawn instead of execSync
function $sh(cmd) {
  return new Promise((resolve, reject) => {
    console.log(`>[async] ${cmd}`);
    const childProcess = exec(cmd);

    // Capture and forward stdout
    childProcess.stdout.on('data', (data) => {
      process.stdout.write(data);
    });

    // Capture and forward stderr
    childProcess.stderr.on('data', (data) => {
      process.stderr.write(data);
    });

    // Handle completion
    childProcess.on('close', (code) => {
      if (code === 0) {
        resolve();
      } else {
        reject(new Error(`Command failed with exit code ${code}`));
      }
    });

    // Handle errors
    childProcess.on('error', (error) => {
      reject(error);
    });
  });
}

exports.$sh = $sh;
