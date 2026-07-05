#!/usr/bin/env python3
import argparse
import os
import shutil
import stat
import tarfile
import zipfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def copy_root(stage: Path) -> None:
    if stage.exists():
        shutil.rmtree(stage)
    shutil.copytree(ROOT / "packaging" / "root", stage, symlinks=True)


def merge_koboroot(stage: Path, archive: Path) -> None:
    with tarfile.open(archive, "r:gz") as tar:
        for member in tar.getmembers():
            target = (stage / member.name).resolve()
            if not str(target).startswith(str(stage.resolve()) + os.sep):
                raise SystemExit(f"unsafe path in {archive}: {member.name}")
        tar.extractall(stage)


def chmod_exec(path: Path) -> None:
    path.chmod(path.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)


def write_config(stage: Path, include_token: bool) -> None:
    matter_dir = stage / "mnt" / "onboard" / ".adds" / "matter"
    sample = matter_dir / "config.env.sample"
    config = matter_dir / "config.env"
    if include_token:
        token = os.environ.get("MATTER_TOKEN", "").strip()
        if not token:
            raise SystemExit("MATTER_TOKEN must be set when --include-token is used")
        text = sample.read_text()
        text = text.replace("MATTER_TOKEN=\n", f"MATTER_TOKEN={token}\n")
        config.write_text(text)


def stage_binary(stage: Path, binary: Path) -> None:
    target_dir = stage / "mnt" / "onboard" / ".adds" / "matter" / "bin"
    target_dir.mkdir(parents=True, exist_ok=True)
    target = target_dir / "matter-kobo-sync"
    shutil.copy2(binary, target)
    chmod_exec(target)


def finish_stage(stage: Path) -> None:
    for rel in [
        "mnt/onboard/.adds/matter/run-sync.sh",
        "mnt/onboard/.adds/matter/run-daemon.sh",
        "mnt/onboard/.adds/matter/stop-daemon.sh",
        "mnt/onboard/.adds/matter/status.sh",
        "etc/init.d/matter-kobo",
    ]:
        chmod_exec(stage / rel)

    rc_dir = stage / "etc" / "rcS.d"
    rc_dir.mkdir(parents=True, exist_ok=True)
    link = rc_dir / "S99matter-kobo"
    if link.exists() or link.is_symlink():
        link.unlink()
    link.symlink_to("../init.d/matter-kobo")


def add_tree_to_tar(tar: tarfile.TarFile, stage: Path) -> None:
    for path in sorted(stage.rglob("*")):
        rel = path.relative_to(stage)
        tar.add(path, arcname=str(rel), recursive=False)


def write_archives(stage: Path, dist: Path) -> None:
    dist.mkdir(parents=True, exist_ok=True)
    koboroot = dist / "KoboRoot.tgz"
    with tarfile.open(koboroot, "w:gz") as tar:
        add_tree_to_tar(tar, stage)

    usb_zip = dist / "matter-kobo-usb-copy.zip"
    with zipfile.ZipFile(usb_zip, "w", compression=zipfile.ZIP_DEFLATED) as zf:
        zf.write(koboroot, ".kobo/KoboRoot.tgz")

    helper_src = ROOT / "scripts" / "install-to-mounted-kobo.sh"
    helper_dst = dist / "install-to-mounted-kobo.sh"
    if helper_src.exists():
        shutil.copy2(helper_src, helper_dst)
        chmod_exec(helper_dst)

    for helper_name in [
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
        helper_src = ROOT / "scripts" / helper_name
        helper_dst = dist / helper_name
        if helper_src.exists():
            shutil.copy2(helper_src, helper_dst)
            chmod_exec(helper_dst)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True, type=Path)
    parser.add_argument(
        "--merge-koboroot",
        action="append",
        default=[],
        type=Path,
        help="existing KoboRoot.tgz payload to merge into this package",
    )
    parser.add_argument("--include-token", action="store_true")
    args = parser.parse_args()

    binary = args.binary.resolve()
    if not binary.exists():
        raise SystemExit(f"binary not found: {binary}")

    stage = ROOT / "build" / "package-root"
    dist = ROOT / "dist"

    copy_root(stage)
    for archive in args.merge_koboroot:
        archive = archive.resolve()
        if not archive.exists():
            raise SystemExit(f"merge archive not found: {archive}")
        merge_koboroot(stage, archive)
    stage_binary(stage, binary)
    write_config(stage, args.include_token)
    finish_stage(stage)
    write_archives(stage, dist)
    print(f"wrote {dist / 'KoboRoot.tgz'}")
    print(f"wrote {dist / 'matter-kobo-usb-copy.zip'}")


if __name__ == "__main__":
    main()
