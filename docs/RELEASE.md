# Releasing vl

The [Release workflow](../.github/workflows/release.yml) builds every target in
one Linux job. It runs `make check` and cross-compiles with CGO disabled for
macOS, Linux and Windows, each on `x86_64` and `arm64`. The macOS and Linux
archives contain `vl` in `.tar.gz` files. Windows archives contain `vl.exe` in
`.zip` files. Each archive also contains the project license and third-party
notices and licenses. Archive names follow `vl_<version>_<os>_<arch>`.
The workflow checks all six archives, runs the Linux x86-64 binary, and uploads
the archives, `install.sh`, `install.ps1`, and `checksums.txt`.

## Rehearse

Once the workflow is on the default branch, run **Release → Run workflow** in
GitHub Actions. Download `vl-release` from the completed run. A manual run
from a branch uses `0.0.0-dev` and produces only the workflow artifact.

To generate the same set of files locally with Bash, `tar`, `zip` and `shasum` available, run:

```sh
make check
make package VERSION=0.0.0-dev
(cd dist && shasum -a 256 -c checksums.txt)
```

## Prepare and publish a release

1. Confirm the release commit is on `origin/main`. Create and push one annotated
   tag for the intended version. For example:

   ```sh
   git fetch origin main
   git tag -a v0.1.0 origin/main -m "Release v0.1.0"
   git push origin refs/tags/v0.1.0
   ```

2. Wait for the Release workflow to succeed. Open the draft in GitHub Releases
   and check that all six archives, both installers, and `checksums.txt` are attached.
   The tag sets the value printed by `vl -version`, without the leading `v`.
   Tags such as `v0.1.0-rc.1` create prerelease drafts.
3. Review the generated notes, update them for users, and publish the draft
   from GitHub. If uploading fails, rerun the failed job before publishing. It
   can refresh the assets on an existing draft.

GitHub Release visibility follows the repository's visibility.
