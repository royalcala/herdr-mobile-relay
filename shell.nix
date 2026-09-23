# Local browser-test toolchain.
#
# Playwright's browsers come from nixpkgs rather than from Playwright's CDN: the
# nix store binaries carry the shared libraries NixOS does not ship in /usr/lib,
# and nothing is downloaded at test time. `PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD`
# stops the runner from fetching its own copies behind our back.
#
#   nix-shell                 # enter the shell
#   make frontend-browser-local
#
# The runner version has to match the browsers: Playwright looks for an exact
# browser revision (chromium-1194, webkit-2215 …) under
# PLAYWRIGHT_BROWSERS_PATH, and nixpkgs' playwright-driver is a specific release.
# scripts/local-browser-tests.sh installs the matching @playwright/test into a
# cache directory outside the repo, so the pinned devDependency stays untouched.
{
  pkgs ? import <nixpkgs> { },
}:

let
  browsers = pkgs.playwright-driver.browsers;
in
pkgs.mkShell {
  packages = [
    pkgs.nodejs_22
    pkgs.playwright-driver.browsers
  ];

  PLAYWRIGHT_BROWSERS_PATH = "${browsers}";
  PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD = "1";

  shellHook = ''
    echo "Playwright browsers: $PLAYWRIGHT_BROWSERS_PATH"
    echo "Run the phone's browser journeys with: make frontend-browser-local"
  '';
}
