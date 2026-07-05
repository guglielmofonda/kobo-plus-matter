#!/usr/bin/env python3
import argparse
import io
import tarfile
from pathlib import Path


def norm_name(name: str) -> str:
    while name.startswith("./"):
        name = name[2:]
    return name.rstrip("/")


def safe_members(archive: Path) -> list[tarfile.TarInfo]:
    with tarfile.open(archive, "r:gz") as tar:
        members = tar.getmembers()
    for member in members:
        normalized = norm_name(member.name)
        if normalized.startswith("../") or normalized == ".." or member.name.startswith("/"):
            raise SystemExit(f"unsafe path in {archive}: {member.name}")
    return members


def add_member(out: tarfile.TarFile, source: tarfile.TarFile, member: tarfile.TarInfo) -> None:
    fileobj = source.extractfile(member) if member.isfile() else None
    if fileobj is not None:
        data = fileobj.read()
        out.addfile(member, io.BytesIO(data))
    else:
        out.addfile(member)


def combine(base: Path, overlay: Path, output: Path) -> None:
    safe_members(base)
    overlay_members = safe_members(overlay)
    overlay_names = {norm_name(member.name) for member in overlay_members}

    output.parent.mkdir(parents=True, exist_ok=True)
    with tarfile.open(base, "r:gz") as base_tar, tarfile.open(overlay, "r:gz") as overlay_tar, tarfile.open(output, "w:gz") as out:
        for member in base_tar.getmembers():
            normalized = norm_name(member.name)
            if normalized and normalized in overlay_names:
                continue
            add_member(out, base_tar, member)
        for member in overlay_tar.getmembers():
            add_member(out, overlay_tar, member)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base", required=True, type=Path, help="existing KoboRoot.tgz to preserve")
    parser.add_argument("--overlay", required=True, type=Path, help="Matter KoboRoot.tgz to add")
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()

    if not args.base.exists():
        raise SystemExit(f"base archive not found: {args.base}")
    if not args.overlay.exists():
        raise SystemExit(f"overlay archive not found: {args.overlay}")
    combine(args.base, args.overlay, args.output)
    print(f"wrote {args.output}")


if __name__ == "__main__":
    main()

