# The frontend of Memos

Install and build from this directory:

```sh
pnpm install --frozen-lockfile
pnpm build
```

`@usememos/mui@0.1.0-20250601165716` is no longer downloadable from npm.
The dependency in `vendor/` is rebuilt from the unmodified MIT-licensed upstream
source at [3ca98006d0c2fa2c12a4a6d6224780fbbe39f05c](https://github.com/usememos/mui/commit/3ca98006d0c2fa2c12a4a6d6224780fbbe39f05c),
the last commit before that release timestamp. It is a source rebuild, not a
byte-for-byte copy of the unavailable release. The archive includes the source,
license, build configuration, and build dependency lockfile for inspection and
rebuilding with `pnpm install --ignore-workspace` and `pnpm build` after extraction.
Only its version and packaged file list were adjusted to identify the local build.
Keep this archive committed alongside `package.json` and `pnpm-lock.yaml`.

Mobile layout regression tests use headless Chromium to measure real element
bounds (jsdom does not calculate layout). Install the test browser before running
`pnpm test`:

```sh
pnpm exec playwright install chromium
```

An existing Chromium binary can be selected with
`PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH`. The layout test covers 320, 390, 768 and
1280px viewports and checks that report cards stay inside the viewport while the
wide chart scrolls within its own container.
The same browser suite also checks that account, password, token, webhook,
identity provider and server profile dialog fields and actions fit a 320px
viewport at 20px font size. Wide memo tables and block equations must scroll
within their own container.

Media bandwidth regression coverage is in `tests/media-bandwidth.test.tsx`:
audio/video attachments defer loading until playback, and list refreshes retain
existing media elements and playback position. Chromium also checks that initial
rendering makes no media requests and that requesting playback starts them.

Server-side attachment previews for database/local JPEG and PNG files use cached
thumbnails with a maximum dimension of 512px; opening a preview still fetches the
original. Versioned thumbnail cache filenames regenerate older oversized previews
on demand. Database file responses support HTTP byte ranges for media playback;
they still read the underlying blob from the database before serving the range.
External/S3 image URLs retain their existing delivery behavior.
