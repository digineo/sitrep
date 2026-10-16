#!/usr/bin/env python3
"""Release sitrep.

Usage:
    scripts/release.py X.Y.Z   release version X.Y.Z
    scripts/release.py TEST    run the self-tests

Moves the unreleased CHANGELOG entries into a section for the new version,
commits that on main, tags it as a signed vX.Y.Z, and pushes main and the
tag at once. CI then publishes the image for the tag.
"""

import datetime
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
CHANGELOG = ROOT / "CHANGELOG.md"


def run(*cmd):
    try:
        return subprocess.run(
            cmd, cwd=ROOT, check=True, text=True, stdout=subprocess.PIPE
        ).stdout.strip()
    except subprocess.CalledProcessError:
        sys.exit(f"release: {' '.join(cmd)} failed")


def parse_version(v):
    return tuple(int(n) for n in v.split("."))


def release_changelog(text, version, date):
    """Moves the unreleased entries into a section for version."""
    head = "## [Unreleased]\n"
    if head not in text:
        raise ValueError("missing Unreleased section")
    if not text.split(head, 1)[1].split("\n## [", 1)[0].strip():
        raise ValueError("no unreleased changes")
    link = re.search(
        r"^\[Unreleased\]: (\S+)/compare/(\S+)\.\.\.HEAD$", text, re.MULTILINE
    )
    if not link:
        raise ValueError("missing Unreleased link")

    base, prev = link.groups()
    text = text.replace(
        link.group(0),
        "\n".join(
            [
                f"[Unreleased]: {base}/compare/v{version}...HEAD",
                f"[{version}]: {base}/compare/{prev}...v{version}",
            ]
        ),
    )
    return text.replace(head, f"{head}\n## [{version}] - {date}\n", 1)


def sync_main():
    if run("git", "branch", "--show-current") != "main":
        sys.exit("release: not on main")
    if run("git", "status", "--porcelain"):
        sys.exit("release: working tree not clean")
    run("git", "fetch", "--tags", "origin")
    run("git", "merge", "--ff-only", "origin/main")
    if run("git", "rev-parse", "HEAD") != run("git", "rev-parse", "origin/main"):
        sys.exit("release: main has unpushed commits")


def confirm(question):
    try:
        return input(f"{question} [y/N] ") == "y"
    except (EOFError, KeyboardInterrupt):
        print()
        return False


def release(version):
    sync_main()
    tags = run("git", "tag", "--list", "v*").split()
    latest = max(
        (parse_version(t[1:]) for t in tags if re.fullmatch(r"v\d+\.\d+\.\d+", t)),
        default=(0, 0, 0),
    )
    if parse_version(version) <= latest:
        sys.exit(f"release: {version} is not newer than {'.'.join(map(str, latest))}")
    try:
        text = release_changelog(
            CHANGELOG.read_text(),
            version,
            datetime.datetime.now(datetime.UTC).date().isoformat(),
        )
    except ValueError as e:
        sys.exit(f"release: CHANGELOG.md: {e}")

    CHANGELOG.write_text(text)
    print(run("git", "diff"))
    if not confirm(f"Commit, tag and push v{version}?"):
        run("git", "restore", "CHANGELOG.md")
        sys.exit("release: aborted")

    tag = f"v{version}"
    run("git", "commit", "--all", "--message", f"changelog: release {version}")
    run("git", "tag", "--sign", "--message", f"sitrep {version}", tag)
    run("git", "push", "--atomic", "origin", "main", tag)
    print(f"Pushed main and {tag}")


def main(argv):
    if len(argv) != 1 or not re.fullmatch(r"\d+\.\d+\.\d+", argv[0]):
        sys.exit(__doc__ or "usage: scripts/release.py X.Y.Z")
    release(argv[0])


def test():
    changelog = """# Changelog

## [Unreleased]

### Added

- New feature.

## [0.2.1] - 2026-10-12

- Old feature.

[Unreleased]: https://github.com/digineo/sitrep/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/digineo/sitrep/compare/v0.2.0...v0.2.1
"""
    released = """# Changelog

## [Unreleased]

## [0.3.0] - 2026-10-16

### Added

- New feature.

## [0.2.1] - 2026-10-12

- Old feature.

[Unreleased]: https://github.com/digineo/sitrep/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/digineo/sitrep/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/digineo/sitrep/compare/v0.2.0...v0.2.1
"""
    for name, text, expected in [
        ("release", changelog, released),
        (
            "no changes",
            changelog.replace("### Added\n\n- New feature.\n\n", ""),
            "no unreleased changes",
        ),
        (
            "no section",
            changelog.replace("## [Unreleased]\n", ""),
            "missing Unreleased section",
        ),
        (
            "no link",
            changelog.replace("[Unreleased]:", "[unreleased]:"),
            "missing Unreleased link",
        ),
    ]:
        try:
            got = release_changelog(text, "0.3.0", "2026-10-16")
        except ValueError as e:
            got = str(e)
        assert got == expected, f"{name}: {got!r}"
    print("ok")


if __name__ == "__main__":
    if sys.argv[1:] == ["TEST"]:
        test()
    else:
        main(sys.argv[1:])
