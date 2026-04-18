# Apt repository metadata for Lara Nux

This directory holds the metadata templates for publishing signed Ubuntu
packages after `dpkg-buildpackage` has produced `.deb`, `.changes`, and `.dsc`
artifacts.

## Intended publish flow

1. Build the package with the Debian metadata from `packaging/ubuntu/debian/`.
2. Sign the `.changes` file with the release GPG key.
3. Generate `Packages`, `Contents`, and `Release` metadata with
   `apt-ftparchive generate packaging/ubuntu/repo/apt-ftparchive.conf`.
4. Sign `Release` into `InRelease` and `Release.gpg`.
5. Publish the repo root so Ubuntu clients can add it as an apt source.

The `distributions` file mirrors the suite/component/architecture intent used by
common repo managers such as `reprepro`.
