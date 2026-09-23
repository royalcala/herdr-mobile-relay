# Local development

How to build, run, and test this project from a checkout, and how to fix the
local runtime problems that come up while doing it. Read this if you are changing
the relay rather than using it.

## Running from a checkout

```bash
git clone https://github.com/0cv/herdr-mobile-relay.git
cd herdr-mobile-relay
make dev-tunnel
```

`make dev-tunnel` builds the current Go source and frontend, uses isolated ports
and state under `relay/.dev/`, and opens a temporary tunnel. It never uses the
installed production relay.

## Common targets

```bash
make check             # all backend, frontend, browser, and release checks
make backend-check     # format, vet, tests, race detector, shell checks
make web-release       # replace committed web/ with a verified frontend build
make web-release-check # compare and browser-test the shipped web/ bundle
make relay-plugin      # link this checkout as a Herdr plugin
make stable-setup      # run the stable tunnel wizard with the installed relay
```

## Committed web bundle

The phone app ships from `web/`, a committed copy of a verified `frontend/dist`
build. Release CI checks that bundle rather than regenerating it, so a frontend
change only reaches phones once `web/` is rebuilt and committed alongside it.

```bash
make web-release        # bump assets, run frontend-check, copy dist -> web, verify
make web-release-check  # assert dist == web and browser-test the shipped bundle
```

`make web-release` runs, in order:

1. `bun frontend/scripts/bump-assets.mjs` — increments the `assets` cache-buster
   in `frontend/build-versions.json`. The product version itself is
   `herdr-plugin.toml`'s `version`, not this counter.
2. `make frontend-check` — eslint, `svelte-check`, `vitest`, the production
   build into `frontend/dist`, the payload size check, and the service-worker
   and notification-icon builds.
3. `bun frontend/scripts/release.mjs` — replaces the committed `web/` tree with
   `frontend/dist`.
4. `make web-bundle-check` — `validate-build.mjs`, `check-size.mjs`, and the
   worker/icon builds against `web/`.

The same result, step by step, when the full check cannot run in one pass:

```bash
bun frontend/scripts/bump-assets.mjs
bun run --cwd frontend build     # vite build + finalize + brotli + validate
bun frontend/scripts/release.mjs # copy frontend/dist -> web
make web-bundle-check
```

Constraints the checks enforce:

- The initial payload — `index.html`, `herdr-bootstrap.js`, the build entry, and
  `assets/app-*.js` / `assets/app-*.css` — must fit a 165 KiB gzip ceiling
  (`frontend/scripts/check-size.mjs`). Keep anything large in a lazy chunk.
- `web/release.json` and `web/version.json` must agree with each other, with
  `herdr-plugin.toml`, and with `frontend/build-versions.json`. The entry path is
  `builds/<version>-<assets>-<build16>/index.html`, and its JS/CSS sha256 and
  Brotli sidecars must match the emitted files.
- `make web-release-check` proves `frontend/dist` and `web/` are byte-identical
  (`diff -qr frontend/dist web`) before the browser suites run against `web/`.

## Testing a release candidate

Candidates are published as prereleases, which ordinary relays never install:
their update check resolves the latest stable release only. To run one:

```bash
herdr plugin install 0cv/herdr-mobile-relay --ref dev
```

Rerun that command to move to a newer candidate.

## Contributing

Work lands on `dev`; open pull requests against it and make sure `make check`
passes first.

## Toolchains

Backend development uses Go 1.27.0; frontend development uses Bun 1.4 (`bun
install --cwd frontend`, then the `make` targets above). Playwright runs on
Bun. CI installs both browsers natively (`bun x playwright install
--with-deps chromium webkit`); on Fedora, `install-deps` is unsupported and
native WebKit crashes, so `make frontend-browser` runs WebKit through
Playwright's official container via podman (Chromium runs natively — its dnf
dependencies are nspr nss dbus-libs atk at-spi2-atk cups-libs at-spi2-core
libXcomposite libXdamage libXext libXfixes libXrandr mesa-libgbm cairo pango
alsa-lib, per passportxyz/passport's fedora-install-playwright-deps.sh).
Publishing the hosted web app (`make web-deploy`,
`make web-preview`) shells out to `npx wrangler`, which requires Node.js 22 or
newer on that computer only; CI and the relay's deploy action are exercised on
Node.js 26. `make web-deploy` then runs the public bundle verifier against
`WEB_ORIGIN` (the Pages domain by default; override it for a custom domain).
Packaged users need no toolchain at all.

### WebKit tests on Fedora

Do not install the Ubuntu-specific `libicu74` / `libjpeg-turbo8` packages or
symlink Fedora libraries to their ABI names. The version-matched official
Playwright container supplies WebKit and its dependencies. Podman must be
installed once (`sudo dnf install podman`); the image is downloaded on first
use and remains cached across runs and reboots. A Playwright version upgrade
fetches the matching new image.

Both browser test commands select the container automatically on Fedora:

```bash
make frontend-browser                    # Chromium and WebKit UI journeys
make frontend-browser-attention-release  # Chromium and WebKit relay/attention tests
# Focus only on the previously blocked engine:
HERDR_WEB_ROOT=../web bun run --cwd frontend test:browser:attention --project=webkit-attention
```

The attention runner keeps Bun, Go, and the isolated relay fixture on the host.
Only the WebKit browser runs in the container, with Playwright forwarding its
loopback traffic to the host's test HTTP and relay WebSocket servers. The
browser-control port is published only on `127.0.0.1`, on an automatically
allocated port, and the runner removes its container on exit without removing
the cached image. Test output and failure traces stay on the host. Ubuntu CI
continues to use native browsers; `HERDR_WEBKIT_CONTAINER=1` selects Docker for
hosts that explicitly want containerized WebKit. Directly invoking
`playwright test --config playwright.attention.config.ts` bypasses the wrapper;
use the package script or Make target instead.

The test-only `cmd/fake-herdr` binary provides deterministic Herdr CLI behavior,
failure injection, and process-control traces for black-box tests.

Installed-PWA device CI is documented in `docs/mobile-device-ci.md`. Its host-only
check does not replace the real Android Home Screen or iOS Home Screen runs;
macOS/Xcode is required for iOS, and each destructive device action requires a
run-owned disposable emulator or simulator marker.

## Herdr compatibility checks

The relay's minimum supported Herdr client is 0.7.5; 0.9.0 is the recommended
client for the full JSON inventory and workspace-management surface. The
installed client version is only one input: startup and the refresh loop ping
the running server and record its server version, protocol, endpoint generation,
and individual feature evidence. A stable endpoint generation does not imply
that every optional operation is supported.

Ordinary agent, pane, workspace, and tab inventory uses JSON operations. The
mobile terminal reads pane snapshots through `pane.read`, with a CLI fallback;
it does not attach through Herdr's separate binary direct-terminal transport.
Unprobed or unadvertised optional features are not compatibility failures.
Settings warns only for unsupported features and unsuccessful checks, not
`not_checked` or `not_advertised` evidence. Terminal-read support is checked at
startup and after reconnects using an empty explicit pane ID: Herdr's
`pane_not_found` refusal confirms the method without reading, scrolling, or
resizing a live pane. Pending reconnect checks are labeled as rechecks, not
failures. Event clients subscribe before taking a snapshot; reconnects refresh
the snapshot and do not replay all notifications missed while disconnected.

Workspace group close is a single explicit close operation over the current
workspace membership. It closes panes but never removes Git checkouts or
branches. Worktree removal remains a separate destructive operation with its
own dirty-checkout confirmation.

Use the fake Herdr binary or a temporary Unix socket fixture for tests. Do not
run production Herdr commands or mutate production state while checking these
paths.

## Phone-side crash diagnostics

The production frontend installs raw DOM handlers before Svelte mounts. An
uncaught exception or rejected promise appears in a bottom **App error** banner;
tap it to dismiss it and allow a later error to be shown. Phones usually have
no accessible console, so include that text in a bug report.

For local `make dev-tunnel` diagnosis, set `HERDR_DEV_RUNTIME=1` before the
build. This enables Svelte's development runtime so invariant failures include
their data and indexes in the on-device banner. Release builds leave it off.

## Troubleshooting local runs

- **Port is busy:** `make dev-tunnel` uses 18375, Quick Start and the installed
  service use 8375; stop whatever already holds the one you need.
- **Herdr is not running:** start it with `herdr`, then retry the operation.
- **Agents are unavailable:** inspect `/healthz`; after a Herdr protocol update,
  run `herdr server live-handoff` and wait for the next relay poll.

## The task board and the herdr session the relay mirrors

`queue/tasks.json` is the versioned task board the phone's Board tab shows. The
relay reads it (path from `HERDR_QUEUE_PATH`, default `queue/tasks.json` relative
to the working directory), sends it over the authenticated WebSocket, and
re-broadcasts it within ten seconds of the file changing. The phone crosses each
row with the live herdr agents by owner name, by the branch checked out in the
agent's working directory (herdr worktrees are named after the branch), or by the
repository in its path; anything that matches nothing is listed as "off the
board" rather than hidden.

Herdr keeps every named session on its own socket. The relay mirrors one at a
time — the session behind `HERDR_SOCKET_PATH`, the default session unless
configured otherwise — and the Sessions tab lists the others and moves the relay
onto one with `select_session`. A switch re-points the socket API, the CLI
environment and the event stream, and drops the previous session's inventory, so
the phone never shows the old session's agents as if they were the new one's.
