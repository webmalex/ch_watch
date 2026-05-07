# DOCS DOMAIN GUIDE

## OVERVIEW
`docs/` stores current operational guidance, not session planning. Every document here should reflect behavior that exists in the repo now.

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Why the project exists | `PROJECT_RATIONALE.md` | durable motivation and design intent |
| Build and install flow | `INSTALL.md` | binary build, install, hook setup |
| Validation workflow | `QUALITY.md` | fast/full/manual checks and hook stages |
| User quick start | `../README.md` | public entry doc; keep examples synchronized |

## CONVENTIONS
- Write docs in Russian, but keep technical terms, binary names, flags, and commands in English.
- Prefer current verified workflows over historical implementation notes.
- When CLI defaults or behavior change, update `README.md`, `INSTALL.md`, and `QUALITY.md` together.
- Keep examples executable against the current Makefile and current CLI flags.

## ANTI-PATTERNS
- Do not keep stale "next session" planning docs as canonical documentation.
- Do not reference removed defaults such as `clickhouse-client` when the code now uses `clickhouse`.
- Do not document checks or commands that have not been locally verified.
- Do not duplicate whole sections from root docs when a short link or pointer is enough.

## COMMANDS
```bash
make build
make check
make check-full
make release-check
make hooks-install
```
