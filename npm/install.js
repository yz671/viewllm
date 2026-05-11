const fs = require("fs");
const path = require("path");
const https = require("https");

const VERSION = require("./package.json").version;
const REPO = "yz671/viewllm";

const PLATFORM_MAP = { linux: "linux", darwin: "darwin", win32: "windows" };
const ARCH_MAP = { x64: "amd64", arm64: "arm64" };

const platform = PLATFORM_MAP[process.platform];
const arch = ARCH_MAP[process.arch];

if (!platform || !arch) {
  console.error(`viewllm: unsupported platform ${process.platform}-${process.arch}`);
  process.exit(1);
}

const ext = platform === "windows" ? ".exe" : "";
const binaryName = `viewllm-${platform}-${arch}${ext}`;
const url = `https://github.com/${REPO}/releases/download/v${VERSION}/${binaryName}`;
const dest = path.join(__dirname, "bin", `viewllm${ext}`);

function download(url, dest, redirects) {
  if (redirects === 0) {
    return Promise.reject(new Error("Too many redirects"));
  }
  return new Promise((resolve, reject) => {
    https.get(url, { headers: { "User-Agent": "viewllm-npm" } }, (res) => {
      if (res.statusCode === 302 || res.statusCode === 301) {
        return download(res.headers.location, dest, redirects - 1).then(resolve, reject);
      }
      if (res.statusCode !== 200) {
        return reject(new Error(`Download failed: HTTP ${res.statusCode} from ${url}`));
      }
      const file = fs.createWriteStream(dest, { mode: 0o755 });
      res.pipe(file);
      file.on("finish", () => { file.close(resolve); });
      file.on("error", reject);
    }).on("error", reject);
  });
}

fs.mkdirSync(path.dirname(dest), { recursive: true });

console.log(`Downloading viewllm v${VERSION} for ${platform}-${arch}...`);
download(url, dest, 5)
  .then(() => console.log("viewllm installed successfully."))
  .catch((err) => {
    console.error(`Failed to install viewllm: ${err.message}`);
    console.error(`You can download manually from: https://github.com/${REPO}/releases`);
    process.exit(1);
  });
