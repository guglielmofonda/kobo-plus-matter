#!/usr/bin/env python3
import sys
import tarfile
from pathlib import Path


PATCH_NAMES = [
    "Remove recommendations from home screen",
    "Hide bottom home row",
    "Remove footer row on new home screen",
    "Change Browse Kobo home screen link target - Activity",
    "Never show Kobo Plus, wishlist, and points SmartLinks",
]


def fail(message: str) -> None:
    print(f"verify-home-cleanup-patch: {message}", file=sys.stderr)
    raise SystemExit(1)


def verify_tar(path: Path) -> None:
    if not path.exists():
        fail(f"missing {path}")
    with tarfile.open(path, "r:gz") as tar:
        names = {member.name.lstrip("./") for member in tar.getmembers()}
    if "usr/local/Kobo/libnickel.so.1.0.0" not in names:
        fail(f"{path} does not contain usr/local/Kobo/libnickel.so.1.0.0")
    if "usr/local/Kobo/nickel" not in names:
        fail(f"{path} does not contain usr/local/Kobo/nickel")


def main() -> None:
    if len(sys.argv) != 4:
        fail("usage: verify-home-cleanup-patch.py KoboRoot.home-cleanup.tgz KoboRoot.home-cleanup-restore.tgz kobopatch.log")

    patch_archive = Path(sys.argv[1])
    restore_archive = Path(sys.argv[2])
    log = Path(sys.argv[3])

    verify_tar(patch_archive)
    verify_tar(restore_archive)

    log_text = log.read_text()
    for patch_name in PATCH_NAMES:
        if patch_name not in log_text:
            fail(f"patch log does not mention {patch_name!r}")
    if "Error:" in log_text:
        fail("patch log contains an error")

    print("verify-home-cleanup-patch: ok")


if __name__ == "__main__":
    main()
