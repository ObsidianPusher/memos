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
