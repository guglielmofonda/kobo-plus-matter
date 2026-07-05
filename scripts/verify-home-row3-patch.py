#!/usr/bin/env python3
import sys
import tarfile
from pathlib import Path


def fail(message: str) -> None:
    print(f"verify-home-row3-patch: {message}", file=sys.stderr)
    raise SystemExit(1)


def main() -> None:
    if len(sys.argv) != 3:
        fail("usage: verify-home-row3-patch.py KoboRoot.home-row3.tgz kobopatch.log")

    archive = Path(sys.argv[1])
    log = Path(sys.argv[2])
    if not archive.exists():
        fail(f"missing archive: {archive}")
    if not log.exists():
        fail(f"missing log: {log}")

    log_text = log.read_text(errors="replace")
    if "Remove Kobo Shop footer row on new home screen" not in log_text:
        fail("kobopatch log does not mention the home row patch")
    if "APPLY" not in log_text and "enabled" not in log_text.lower():
        fail("kobopatch log does not show an applied patch")

    with tarfile.open(archive, "r:gz") as tar:
        members = {member.name.lstrip("./"): member for member in tar.getmembers()}

    target = "usr/local/Kobo/libnickel.so.1.0.0"
    if target not in members:
        fail(f"archive missing {target}")
    member = members[target]
    if member.size < 10_000_000:
        fail(f"{target} is unexpectedly small")

    print("verify-home-row3-patch: ok")


if __name__ == "__main__":
    main()
