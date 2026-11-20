## Summary

<!-- Describe the problem, the change, and any issue it closes. -->

## Verification

<!-- Check what you ran and replace the examples when a narrower check is appropriate. -->

- [ ] Focused tests covering the changed behavior passed.
- [ ] `GOTOOLCHAIN=go1.27.4 make check` passed.
- [ ] `make build` passed.
- [ ] For collection/report changes, the real CLI scenario was exercised and per-collector errors were inspected.

## Compatibility and documentation

- [ ] Collection remains read-only and existing cancellation/partial-report behavior is preserved.
- [ ] The JSON report remains compatible, or the schema version, tests, examples, and documentation were updated together.
- [ ] User-visible changes are reflected in README and the affected English and Russian documentation.
- [ ] No generated binaries, reports, credentials, or local tool configuration are included.

## Environment-dependent checks

<!-- State whether Docker integration, privileged validation, or release-only checks were not run and why. -->
