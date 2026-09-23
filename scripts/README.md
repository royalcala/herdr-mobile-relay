# scripts

## Local browser tests

The phone's browser journeys can run on this machine, with no CI and no browser
download:

```bash
make frontend-browser-local                     # the bundle the relay serves (web/)
make frontend-browser-local WEB_ROOT=frontend/dist
make frontend-browser-local PW_ARGS='-g "Ctrl"'   # a single journey
nix-shell                                       # the same environment, interactively
```

`scripts/local-browser-tests.sh [web-root] [playwright args…]` is the whole
recipe; the make target is a thin wrapper.

### How it works, and why it is shaped this way

- **The browsers come from nixpkgs** (`pkgs.playwright-driver.browsers`), pointed
  at by `PLAYWRIGHT_BROWSERS_PATH`, with `PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1` so
  the runner never fetches its own copies. Those binaries carry the shared
  libraries NixOS does not ship in `/usr/lib`, which is why Playwright's own
  Chromium fails to launch here with "Target page, context or browser has been
  closed".
- **The runner version must match the browsers.** Playwright looks for an exact
  browser revision (`chromium-1194`, `webkit-2215` …) under
  `PLAYWRIGHT_BROWSERS_PATH`, and nixpkgs' `playwright-driver` is one specific
  release (1.56.1 as of writing). The script reads that version from the nix
  store path and installs the matching `@playwright/test` into
  `$XDG_CACHE_HOME/herdr-mobile-relay/browser-harness`, **outside the repo**: the
  `devDependency` pinned in `frontend/package.json` is what CI uses and is left
  alone. `NODE_ENV=production` in a parent shell makes npm skip devDependencies,
  so the install asks for `--include=dev` explicitly.
- **The specs are copied, not symlinked.** Node resolves `@playwright/test` from
  the importing file's own directory, so a symlink into the repo would load the
  repo's pinned runner instead of the one that matches these browsers.
- **Chromium is the default; WebKit is opt-in.** nixpkgs ships WebKit as an
  ubuntu-20.04 tree (`webkit_ubuntu20.04_x64_special`), which this runner cannot
  launch on NixOS:

  ```
  Executable doesn't exist at …/playwright-browsers/webkit_ubuntu20.04_x64_special-2092/pw_run.sh
  ```

  So the local gate runs Chromium (Pixel 7). To try WebKit anyway:

  ```bash
  HERDR_PW_PROJECTS=chromium-mobile,webkit-mobile scripts/local-browser-tests.sh web
  ```

  CI still runs both engines natively; this recipe is for verifying a change here
  before pushing, without waiting for a workflow.

### Result on this machine

`make frontend-browser-local` against the committed bundle: **105 passed, 0
failed** (Chromium, Pixel 7, 4 workers, 56s). WebKit does not run here for
the reason above.
