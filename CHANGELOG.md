# Changelog

All notable Threadwave changes are documented here. Threadwave follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and will use semantic
versioning once releases begin.

For the history of the imported foundation through v1.16.0, see the
[upstream ygo changelog](https://github.com/Deln0r/ygo/blob/main/CHANGELOG.md).
Threadwave starts a new release history and does not reuse upstream tags.

## Unreleased

### Added

- Configurable server logging through `server.Options.Logger`.
- Independent `github.com/Arnavsharma2/threadwave` module and package identity.
- Branded `threadserve` server and `threadload` load-test commands.
- Explicit upstream provenance in `NOTICE.md` and the README.

### Changed

- All public imports, package names, examples, mobile artifact names, and
  documentation now use the Threadwave identity.
- Project maintenance, security reporting, CI links, and contribution guidance
  now point to the Threadwave repository.

### Removed

- Inherited deployment-specific live-demo and mirror claims.
- The redundant compatibility server binary; `threadserve` is the single
  supported standalone server command.
