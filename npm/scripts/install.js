#!/usr/bin/env node
"use strict";

const { execSync } = require("child_process");
const fs = require("fs");
const https = require("https");
const os = require("os");
const path = require("path");
const crypto = require("crypto");

const REPO = "inovacc/thimble";
// Binary goes to package root — CLAUDE_PLUGIN_ROOT points here.
const PKG_DIR = path.join(__dirname, "..");

function getPlatform() {
  const p = os.platform();
  switch (p) {
    case "darwin": return "Darwin";
    case "linux": return "Linux";
    case "win32": return "Windows";
    default: throw new Error(`Unsupported platform: ${p}`);
  }
}

function getArch() {
  const a = os.arch();
  switch (a) {
    case "x64": return "x86_64";
    case "arm64": return "arm64";
    default: throw new Error(`Unsupported architecture: ${a}`);
  }
}

function binaryName() {
  return os.platform() === "win32" ? "thimble.exe" : "thimble";
}

function assetName(platform, arch) {
  const ext = platform === "Windows" ? ".zip" : ".tar.gz";
  return `thimble_${platform}_${arch}${ext}`;
}

function fetch(url) {
  return new Promise((resolve, reject) => {
    const get = (u, redirects = 0) => {
      if (redirects > 5) return reject(new Error("Too many redirects"));
      https.get(u, { headers: { "User-Agent": "thimble-npm-installer" } }, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          return get(res.headers.location, redirects + 1);
        }
        if (res.statusCode !== 200) {
          return reject(new Error(`HTTP ${res.statusCode} for ${u}`));
        }
        const chunks = [];
        res.on("data", (c) => chunks.push(c));
        res.on("end", () => resolve(Buffer.concat(chunks)));
        res.on("error", reject);
      }).on("error", reject);
    };
    get(url);
  });
}

async function fetchJSON(url) {
  const buf = await fetch(url);
  return JSON.parse(buf.toString());
}

async function downloadToFile(url, dest) {
  const buf = await fetch(url);
  fs.writeFileSync(dest, buf);
}

function findBinary(dir, name) {
  // Search for the binary in the directory (flat or nested archive layouts).
  const direct = path.join(dir, name);
  if (fs.existsSync(direct)) return direct;

  // Search one level deep (goreleaser may nest in a directory).
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      const nested = path.join(dir, entry.name, name);
      if (fs.existsSync(nested)) return nested;
    }
  }

  throw new Error(`Binary ${name} not found in ${dir}`);
}

async function main() {
  const binPath = path.join(PKG_DIR, binaryName());
  if (fs.existsSync(binPath)) {
    console.log("thimble binary already exists, skipping download");
    return;
  }

  const plat = getPlatform();
  const arch = getArch();
  const asset = assetName(plat, arch);

  console.log(`Installing thimble for ${plat}/${arch}...`);

  // Fetch release matching package version.
  const pkg = require("../package.json");
  const version = pkg.version.startsWith("v") ? pkg.version : `v${pkg.version}`;
  const releaseURL = `https://api.github.com/repos/${REPO}/releases/tags/${version}`;

  let release;
  try {
    release = await fetchJSON(releaseURL);
  } catch (e) {
    console.warn(`Release ${version} not found, trying latest: ${e.message}`);
    release = await fetchJSON(`https://api.github.com/repos/${REPO}/releases/latest`);
  }

  const assetObj = release.assets.find((a) => a.name === asset);
  if (!assetObj) {
    throw new Error(`Asset ${asset} not found in release ${release.tag_name}`);
  }

  // Download archive.
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "thimble-"));
  try {
    const archivePath = path.join(tmpDir, asset);

    console.log(`Downloading ${assetObj.browser_download_url}...`);
    await downloadToFile(assetObj.browser_download_url, archivePath);

    // Download checksums.
    const checksumAsset = release.assets.find((a) =>
      a.name.toLowerCase().includes("checksum") || a.name.toLowerCase().includes("sha256")
    );

    if (checksumAsset) {
      const checksumBuf = await fetch(checksumAsset.browser_download_url);
      const checksums = checksumBuf.toString();
      const fileBuf = fs.readFileSync(archivePath);
      const hash = crypto.createHash("sha256").update(fileBuf).digest("hex");

      const expected = checksums.split("\n").find((l) => l.includes(asset));
      if (expected) {
        const expectedHash = expected.trim().split(/\s+/)[0];
        if (expectedHash !== hash) {
          throw new Error(`Checksum mismatch for ${asset}: expected ${expectedHash}, got ${hash}`);
        }
      }
      console.log("Checksum verified.");
    }

    // Extract binary to package root.
    if (asset.endsWith(".zip")) {
      execSync(`powershell -Command "Expand-Archive -Path '${archivePath}' -DestinationPath '${tmpDir}/extract' -Force"`, { stdio: "inherit" });
      const extracted = findBinary(path.join(tmpDir, "extract"), binaryName());
      fs.copyFileSync(extracted, binPath);
    } else {
      execSync(`tar xzf "${archivePath}" -C "${tmpDir}"`, { stdio: "inherit" });
      const extracted = findBinary(tmpDir, binaryName());
      fs.copyFileSync(extracted, binPath);
    }

    // Make executable on Unix.
    if (os.platform() !== "win32") {
      fs.chmodSync(binPath, 0o755);
    }

    console.log(`Installed thimble to ${binPath}`);
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

main().catch((err) => {
  console.error("thimble install failed:", err.message);
  process.exit(1);
});
