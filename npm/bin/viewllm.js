#!/usr/bin/env node
const { execFileSync } = require("child_process");
const https = require("https");
const path = require("path");

const currentVersion = require("../package.json").version;

function checkForUpdate() {
  const req = https.get("https://registry.npmjs.org/viewllm/latest", { timeout: 3000 }, (res) => {
    let data = "";
    res.on("data", (chunk) => { data += chunk; });
    res.on("end", () => {
      try {
        const latest = JSON.parse(data).version;
        if (latest && latest !== currentVersion) {
          console.log(`\n  Update available: ${currentVersion} → ${latest}`);
          console.log(`  Run: npx viewllm@latest\n`);
        }
      } catch (e) {}
    });
  });
  req.on("error", () => {});
  req.on("timeout", () => { req.destroy(); });
}

checkForUpdate();

const ext = process.platform === "win32" ? ".exe" : "";
const bin = path.join(__dirname, `viewllm${ext}`);

try {
  execFileSync(bin, process.argv.slice(2), { stdio: "inherit" });
} catch (e) {
  process.exit(e.status || 1);
}
