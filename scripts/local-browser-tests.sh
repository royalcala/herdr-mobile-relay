#!/usr/bin/env bash
# Runs the phone's browser journeys on this machine: no CI, no browser download.
#
# Playwright's browsers come from nixpkgs (their binaries carry the libraries
# NixOS does not ship in /usr/lib) and the runner is installed at the version
# those browsers belong to, because Playwright looks for an exact browser
# revision under PLAYWRIGHT_BROWSERS_PATH. The runner lives in a cache directory
# outside the repo, so the pinned devDependency and the repo tree stay untouched.
#
#   scripts/local-browser-tests.sh [web-root] [playwright args…]
#
# Default web root is ./web, the bundle the relay actually serves. Use
# `frontend/dist` to test a fresh build. Enter `nix-shell` first for the same
# environment interactively; this script resolves the paths itself so it also
# works outside the shell.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
FRONTEND_DIR="$REPO_DIR/frontend"
WEB_ROOT="${1:-$REPO_DIR/web}"
if [ "$#" -gt 0 ]; then
    shift
fi
if [ ! -d "$WEB_ROOT" ]; then
    echo "web root not found: $WEB_ROOT" >&2
    exit 1
fi
WEB_ROOT="$(cd "$WEB_ROOT" && pwd)"

if ! command -v nix-build >/dev/null 2>&1; then
    echo "nix-build is required: this recipe runs Playwright's browsers from nixpkgs." >&2
    exit 1
fi

DRIVER="$(nix-build -E 'with import <nixpkgs> {}; playwright-driver' --no-out-link)"
BROWSERS="$(nix-build -E 'with import <nixpkgs> {}; playwright-driver.browsers' --no-out-link)"
VERSION="$(basename "$DRIVER" | sed -n 's/^.*-playwright-core-\([0-9][0-9.]*\)$/\1/p')"
if [ -z "$VERSION" ]; then
    echo "could not read the runner version from $DRIVER" >&2
    exit 1
fi

HARNESS="${XDG_CACHE_HOME:-$HOME/.cache}/herdr-mobile-relay/browser-harness"
mkdir -p "$HARNESS/tests/browser"

# The runner has to match the browsers, so it is pinned to the nix release.
# The guard checks the installed tree, not just the manifest: an interrupted
# install leaves a package.json that looks right and a runner that is not there.
if [ ! -x "$HARNESS/node_modules/.bin/playwright" ] \
    || ! grep -qs "\"@playwright/test\": \"$VERSION\"" "$HARNESS/package.json" 2>/dev/null; then
    printf '{\n  "private": true,\n  "devDependencies": { "@playwright/test": "%s" }\n}\n' "$VERSION" \
        > "$HARNESS/package.json"
    echo "Installing @playwright/test@$VERSION to match the nix browsers…"
    # --include=dev because NODE_ENV=production in a parent environment would
    # otherwise install nothing and leave the runner missing.
    ( cd "$HARNESS" && npm install --include=dev --silent --no-audit --no-fund )
fi

# The specs are copied rather than symlinked: Node resolves @playwright/test
# from the file's own directory, and a symlink into the repo would load the
# repo's pinned runner instead of the one that matches these browsers.
cp "$FRONTEND_DIR/tests/browser/mobile-journeys.spec.ts" "$HARNESS/tests/browser/"
cp "$FRONTEND_DIR/scripts/browser-server.mjs" "$HARNESS/browser-server.mjs"
PROJECTS="${HERDR_PW_PROJECTS:-chromium-mobile}"
PROJECTS_JSON="["
case ",$PROJECTS," in
    *,chromium-mobile,*) PROJECTS_JSON="$PROJECTS_JSON{ name: 'chromium-mobile', use: { ...devices['Pixel 7'] } }," ;;
esac
case ",$PROJECTS," in
    *,webkit-mobile,*) PROJECTS_JSON="$PROJECTS_JSON{ name: 'webkit-mobile', use: { ...devices['iPhone 15'] } }," ;;
esac
PROJECTS_JSON="${PROJECTS_JSON%,}]"

cat > "$HARNESS/playwright.config.ts" <<CONFIG
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests/browser',
  timeout: 30_000,
  expect: { timeout: 5_000 },
  fullyParallel: true,
  use: {
    baseURL: 'http://127.0.0.1:4173',
    serviceWorkers: 'block',
    trace: 'retain-on-failure',
  },
  projects: ${PROJECTS_JSON},
  webServer: {
    command: \`node \${JSON.stringify('$HARNESS/browser-server.mjs')} \${JSON.stringify('$WEB_ROOT')}\`,
    port: 4173,
    reuseExistingServer: false,
  },
});
CONFIG

# nixpkgs' webkit build is an ubuntu-20.04 tree (webkit_ubuntu20.04_x64_special),
# which this Playwright cannot launch on NixOS: it wants its own webkit-<rev>
# directory and fails with "Executable doesn't exist …/pw_run.sh". Chromium is
# therefore the default and webkit is opt-in, so the local gate is usable.
PROJECTS="${HERDR_PW_PROJECTS:-chromium-mobile}"

export PLAYWRIGHT_BROWSERS_PATH="$BROWSERS"
export PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1
export HERDR_WEB_ROOT="$WEB_ROOT"

echo "Runner @playwright/test@$VERSION · browsers $(basename "$BROWSERS") · bundle $WEB_ROOT"
cd "$HARNESS"
exec ./node_modules/.bin/playwright test "$@"
