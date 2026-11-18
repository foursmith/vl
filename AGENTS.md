# Build

- Use `make build` to build the executable at `bin/vl`.
- The user runs this repository's `bin/vl` through the `~/.local/bin/vl` symlink. Update this build artifact when delivering changes that require runtime verification.
- Do not substitute `go build ./cmd/vl` for the delivery build. It creates `vl` in the repository root and does not update the executable the user runs.
- Running vl processes must be closed and reopened to use the new build.

# Commit Messages

- Write commit messages in English using Conventional Commits: `<type>[optional scope]: <description>`.
- Use a suitable type, such as `feat`, `fix`, `perf`, `refactor`, `docs`, `test`, `build`, `ci`, or `chore`.
- Keep the description concise, in the imperative mood, and without a trailing period.
- Add a body when needed to explain the reason for the change and any important behavior or compatibility implications.
- Example: `fix: preserve terminal when switching workspaces`.