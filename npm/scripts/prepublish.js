#!/usr/bin/env node
"use strict";

/**
 * prepublish.js — copies plugin assets from the repo into the npm package
 * directory before publishing. This ensures the npm package contains the
 * full Claude Code plugin structure (.claude-plugin/, skills/, hooks/,
 * agents/, .mcp.json) alongside the binary installer.
 *
 * Run automatically via prepublishOnly in package.json.
 */

const fs = require("fs");
const path = require("path");

const PKG_DIR = path.join(__dirname, "..");
const REPO_ROOT = path.join(PKG_DIR, "..");

const COPY_MAP = [
  // [source (relative to repo root), dest (relative to npm pkg dir)]
  [".claude-plugin", ".claude-plugin"],
  ["assets/plugin/.mcp.json", ".mcp.json"],
  ["assets/plugin/skills", "skills"],
  ["assets/plugin/hooks", "hooks"],
  ["agents", "agents"],
];

function copyRecursive(src, dest) {
  const stat = fs.statSync(src);

  if (stat.isDirectory()) {
    fs.mkdirSync(dest, { recursive: true });
    for (const entry of fs.readdirSync(src)) {
      // Skip Go embed files — not needed in npm package.
      if (entry === "embed.go") continue;
      copyRecursive(path.join(src, entry), path.join(dest, entry));
    }
  } else {
    fs.mkdirSync(path.dirname(dest), { recursive: true });
    fs.copyFileSync(src, dest);
  }
}

function main() {
  console.log("Copying plugin assets into npm package...");

  for (const [srcRel, destRel] of COPY_MAP) {
    const src = path.join(REPO_ROOT, srcRel);
    const dest = path.join(PKG_DIR, destRel);

    if (!fs.existsSync(src)) {
      console.warn(`  SKIP: ${srcRel} (not found)`);
      continue;
    }

    // Remove previous copy to ensure clean state.
    fs.rmSync(dest, { recursive: true, force: true });

    copyRecursive(src, dest);
    console.log(`  ${srcRel} -> ${destRel}`);
  }

  // Patch plugin.json to use the npm package version.
  const pkgJSON = require("../package.json");
  const version = pkgJSON.version;

  const pluginJSONPath = path.join(PKG_DIR, ".claude-plugin", "plugin.json");
  if (fs.existsSync(pluginJSONPath)) {
    const pluginJSON = JSON.parse(fs.readFileSync(pluginJSONPath, "utf8"));
    pluginJSON.version = version;
    fs.writeFileSync(pluginJSONPath, JSON.stringify(pluginJSON, null, 2) + "\n");
    console.log(`  Patched plugin.json version to ${version}`);
  }

  const marketplaceJSONPath = path.join(PKG_DIR, ".claude-plugin", "marketplace.json");
  if (fs.existsSync(marketplaceJSONPath)) {
    const mktJSON = JSON.parse(fs.readFileSync(marketplaceJSONPath, "utf8"));
    if (mktJSON.metadata) mktJSON.metadata.version = version;
    if (Array.isArray(mktJSON.plugins)) {
      for (const p of mktJSON.plugins) {
        if (p.version) p.version = version;
      }
    }
    fs.writeFileSync(marketplaceJSONPath, JSON.stringify(mktJSON, null, 2) + "\n");
    console.log(`  Patched marketplace.json version to ${version}`);
  }

  console.log("Done.");
}

main();
