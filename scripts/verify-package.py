#!/usr/bin/env python3
import sys
import tarfile
import zipfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
DIST = ROOT / "dist"

REQUIRED_TAR_ENTRIES = {
    "etc/dbus-1/system.d/com-github-shermp-nickeldbus.conf",
    "etc/init.d/matter-kobo",
    "etc/rcS.d/S99matter-kobo",
    "mnt/onboard/.adds/matter/bin/matter-kobo-sync",
    "mnt/onboard/.adds/matter/cacert.pem",
    "mnt/onboard/.adds/matter/config.env.sample",
    "mnt/onboard/.adds/matter/run-daemon.sh",
    "mnt/onboard/.adds/matter/run-sync.sh",
    "mnt/onboard/.adds/matter/status.sh",
    "mnt/onboard/.adds/matter/stop-daemon.sh",
    "mnt/onboard/.adds/nm/doc",
    "mnt/onboard/.adds/nm/matter",
    "mnt/onboard/.adds/nickeldbus",
    "mnt/onboard/Matter/Matter Setup.txt",
    "usr/bin/qndb",
    "usr/local/Kobo/imageformats/libndb.so",
    "usr/local/Kobo/imageformats/libnm.so",
}

EXECUTABLE_ENTRIES = {
    "etc/init.d/matter-kobo",
    "mnt/onboard/.adds/matter/bin/matter-kobo-sync",
    "mnt/onboard/.adds/matter/run-daemon.sh",
    "mnt/onboard/.adds/matter/run-sync.sh",
    "mnt/onboard/.adds/matter/status.sh",
    "mnt/onboard/.adds/matter/stop-daemon.sh",
    "usr/bin/qndb",
    "usr/local/Kobo/imageformats/libndb.so",
    "usr/local/Kobo/imageformats/libnm.so",
}


def fail(message: str) -> None:
    print(f"verify-package: {message}", file=sys.stderr)
    raise SystemExit(1)


def verify_tar() -> None:
    archive = DIST / "KoboRoot.tgz"
    if not archive.exists():
        fail(f"missing {archive}")

    with tarfile.open(archive, "r:gz") as tar:
        members = {member.name: member for member in tar.getmembers()}

    missing = sorted(REQUIRED_TAR_ENTRIES.difference(members))
    if missing:
        fail("missing archive entries: " + ", ".join(missing))

    link = members["etc/rcS.d/S99matter-kobo"]
    if not link.issym() or link.linkname != "../init.d/matter-kobo":
        fail("etc/rcS.d/S99matter-kobo must be a symlink to ../init.d/matter-kobo")

    for entry in sorted(EXECUTABLE_ENTRIES):
        mode = members[entry].mode
        if mode & 0o111 == 0:
            fail(f"{entry} is not executable")

    ca_member = members["mnt/onboard/.adds/matter/cacert.pem"]
    if ca_member.size < 1024:
        fail("mnt/onboard/.adds/matter/cacert.pem is unexpectedly small")


def verify_zip() -> None:
    archive = DIST / "matter-kobo-usb-copy.zip"
    if not archive.exists():
        fail(f"missing {archive}")
    with zipfile.ZipFile(archive) as zf:
        names = set(zf.namelist())
    if names != {".kobo/KoboRoot.tgz"}:
        fail(f"unexpected zip entries: {sorted(names)}")


def verify_combiner_exists() -> None:
    script = ROOT / "scripts" / "combine-koboroot.py"
    if not script.exists():
        fail(f"missing {script}")


def verify_dist_helpers() -> None:
    for name in [
        "install-to-mounted-kobo.sh",
        "verify-mounted-kobo.sh",
        "seed-matter-token.sh",
        "update-mounted-kobo.sh",
        "label-mounted-kobo-matter.sh",
        "clean-mounted-kobo-home.sh",
        "disable-mounted-kobo-store-promos.sh",
        "install-home-cleanup-to-mounted-kobo.sh",
        "install-home-cleanup-restore-to-mounted-kobo.sh",
        "verify-mounted-home-cleanup.sh",
        "finalize-mounted-kobo.sh",
        "wait-for-kobo.sh",
    ]:
        script = DIST / name
        if not script.exists():
            fail(f"missing {script}")
        if script.stat().st_mode & 0o111 == 0:
            fail(f"{script} is not executable")


def main() -> None:
    verify_tar()
    verify_zip()
    verify_combiner_exists()
    verify_dist_helpers()
    print("verify-package: ok")


if __name__ == "__main__":
    main()
