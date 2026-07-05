#!/usr/bin/env python3
import os
import shutil
import subprocess
import tarfile
import tempfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def write_tar(path: Path, files: dict[str, bytes]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with tarfile.open(path, "w:gz") as tar:
        for name, data in files.items():
            info = tarfile.TarInfo(name)
            info.size = len(data)
            info.mode = 0o644
            tar.addfile(info, fileobj=__import__("io").BytesIO(data))


def tar_names(path: Path) -> set[str]:
    with tarfile.open(path, "r:gz") as tar:
        return {member.name.lstrip("./") for member in tar.getmembers()}


def run(cmd: list[str], **kwargs) -> subprocess.CompletedProcess:
    return subprocess.run(cmd, cwd=ROOT, check=True, text=True, capture_output=True, **kwargs)


def test_combine_koboroot() -> None:
    with tempfile.TemporaryDirectory() as td:
        tmp = Path(td)
        base = tmp / "base.tgz"
        overlay = tmp / "overlay.tgz"
        output = tmp / "combined.tgz"
        write_tar(base, {
            "./usr/local/Kobo/nickel": b"official",
            "./same/path": b"base",
        })
        write_tar(overlay, {
            "mnt/onboard/.adds/matter/config.env.sample": b"sample",
            "same/path": b"overlay",
        })

        run([
            "python3",
            "scripts/combine-koboroot.py",
            "--base",
            str(base),
            "--overlay",
            str(overlay),
            "--output",
            str(output),
        ])

        names = tar_names(output)
        assert "usr/local/Kobo/nickel" in names
        assert "mnt/onboard/.adds/matter/config.env.sample" in names
        assert "same/path" in names

        with tarfile.open(output, "r:gz") as tar:
            same = tar.extractfile("same/path")
            assert same is not None
            assert same.read() == b"overlay"


def test_install_helper_preserves_existing_update() -> None:
    with tempfile.TemporaryDirectory() as td:
        tmp = Path(td)
        volume = tmp / "KOBOeReader"
        kobo_dir = volume / ".kobo"
        kobo_dir.mkdir(parents=True)
        existing = kobo_dir / "KoboRoot.tgz"
        write_tar(existing, {"./usr/local/Kobo/nickel": b"official"})

        combined = ROOT / "dist" / "KoboRoot.combined.tgz"
        if combined.exists():
            combined.unlink()

        run(["sh", "scripts/install-to-mounted-kobo.sh", str(volume)])

        backup = kobo_dir / "KoboRoot.before-matter.tgz"
        assert backup.exists()
        assert combined.exists()
        names = tar_names(existing)
        assert "usr/local/Kobo/nickel" in names
        assert "mnt/onboard/.adds/matter/bin/matter-kobo-sync" in names

        combined.unlink()


def test_install_helper_seeds_token() -> None:
    with tempfile.TemporaryDirectory() as td:
        tmp = Path(td)
        volume = tmp / "KOBOeReader"
        (volume / ".kobo").mkdir(parents=True)
        env = os.environ.copy()
        env["MATTER_TOKEN"] = "mat_test_token"

        run(["sh", "scripts/install-to-mounted-kobo.sh", str(volume)], env=env)

        config = volume / ".adds" / "matter" / "config.env"
        assert config.exists()
        assert "MATTER_TOKEN=mat_test_token" in config.read_text()


def test_update_helper_preserves_config() -> None:
    with tempfile.TemporaryDirectory() as td:
        tmp = Path(td)
        volume = tmp / "KOBOeReader"
        (volume / ".kobo").mkdir(parents=True)
        config = volume / ".adds" / "matter" / "config.env"
        config.parent.mkdir(parents=True)
        config.write_text("MATTER_TOKEN=mat_existing\n")

        run(["sh", "scripts/update-mounted-kobo.sh", str(volume)])

        assert config.read_text() == "MATTER_TOKEN=mat_existing\n"
        assert (volume / ".adds" / "matter" / "bin" / "matter-kobo-sync").exists()
        assert (volume / ".adds" / "matter" / "cacert.pem").exists()
        assert (volume / ".adds" / "nm" / "matter").exists()
        assert (volume / "Matter" / "Matter Setup.txt").exists()


def create_minimal_kobo_db(db: Path) -> None:
    run([
        "sqlite3",
        str(db),
        """
        CREATE TABLE Shelf (
          CreationDate TEXT,
          Id TEXT,
          InternalName TEXT,
          LastModified TEXT,
          Name TEXT,
          Type TEXT,
          _IsDeleted BOOL,
          _IsVisible BOOL,
          _IsSynced BOOL,
          _SyncTime TEXT,
          LastAccessed TEXT,
          PRIMARY KEY(Id)
        );
        CREATE TABLE ShelfContent (
          ShelfName TEXT,
          ContentId TEXT,
          DateModified TEXT,
          _IsDeleted BOOL,
          _IsSynced BOOL,
          PRIMARY KEY(ShelfName, ContentId)
        );
        CREATE TABLE content (
          ContentID TEXT NOT NULL,
          ContentType TEXT NOT NULL,
          Title TEXT,
          PRIMARY KEY(ContentID)
        );
        INSERT INTO content VALUES
          ('file:///mnt/onboard/Matter/One.epub', '6', 'One'),
          ('file:///mnt/onboard/Other.epub', '6', 'Other'),
          ('file:///mnt/onboard/Matter/Chapter.epub#(0)OEBPS/article.xhtml', '9', 'Chapter');
        """,
    ])


def test_finalize_helper_runs_post_sync_pass() -> None:
    if shutil.which("sqlite3") is None:
        return
    with tempfile.TemporaryDirectory() as td:
        tmp = Path(td)
        volume = tmp / "KOBOeReader"
        db = volume / ".kobo" / "KoboReader.sqlite"
        db.parent.mkdir(parents=True, exist_ok=True)
        config = volume / ".adds" / "matter" / "config.env"
        config.parent.mkdir(parents=True)
        config.write_text("MATTER_TOKEN=mat_existing\n")
        create_minimal_kobo_db(db)

        result = run(["sh", "scripts/finalize-mounted-kobo.sh", str(volume)])
        assert "Mounted Kobo verification passed" in result.stdout
        assert "Matter collection now contains 1 imported EPUB(s)." in result.stdout
        assert "Kobo imported Matter EPUBs: 1" in result.stdout
        assert "Matter collection entries: 1" in result.stdout


def test_clean_home_helper_disables_store_widgets() -> None:
    if shutil.which("sqlite3") is None:
        return
    with tempfile.TemporaryDirectory() as td:
        tmp = Path(td)
        volume = tmp / "KOBOeReader"
        db = volume / ".kobo" / "KoboReader.sqlite"
        db.parent.mkdir(parents=True, exist_ok=True)
        run([
            "sqlite3",
            str(db),
            """
            CREATE TABLE Activity (
              Id TEXT,
              Enabled BIT default TRUE,
              Type TEXT,
              Action INTEGER,
              Date TEXT,
              Data BLOB,
              PRIMARY KEY(Id, Type)
            );
            INSERT INTO Activity VALUES
              ('store', 'true', 'Bookstore', 2, '2026-01-01T00:00:00', NULL),
              ('rec', 'true', 'Recommendations', 1, '2026-01-01T00:00:00', NULL),
              ('recent', 'true', 'RecentBook', 2, '2026-01-01T00:00:00', NULL);
            """,
        ])

        result = run(["sh", "scripts/clean-mounted-kobo-home.sh", str(volume)])
        assert "2 -> 0 active row(s)" in result.stdout
        active_store = run([
            "sqlite3",
            str(db),
            "SELECT count(*) FROM Activity WHERE Enabled = 'true' AND Type IN ('Bookstore', 'Recommendations');",
        ]).stdout.strip()
        active_recent = run([
            "sqlite3",
            str(db),
            "SELECT count(*) FROM Activity WHERE Enabled = 'true' AND Type = 'RecentBook';",
        ]).stdout.strip()
        assert active_store == "0"
        assert active_recent == "1"


def test_store_promo_helper_disables_kobo_plus_sources() -> None:
    if shutil.which("sqlite3") is None:
        return
    with tempfile.TemporaryDirectory() as td:
        tmp = Path(td)
        volume = tmp / "KOBOeReader"
        db = volume / ".kobo" / "KoboReader.sqlite"
        conf = volume / ".kobo" / "Kobo" / "Kobo eReader.conf"
        conf.parent.mkdir(parents=True)
        conf.write_text(
            "\n".join([
                "[ApplicationPreferences]",
                "KoboPlusPromoShown=false",
                "",
                "[OneStoreServices]",
                "featured_lists=https://storeapi.kobo.com/v1/products/featured",
                "kobo_subscriptions_enabled=True",
                "product_recommendations=https://storeapi.kobo.com/v1/products/{ProductId}/recommendations",
                "store_home=www.kobo.com/{region}/{language}",
                "subs_landing_page=https://www.kobo.com/{region}/{language}/plus",
                "subs_plans_page=https://www.kobo.com/{region}/{language}/plus/plans",
                "user_recommendations=https://storeapi.kobo.com/v1/user/recommendations",
                "",
            ])
        )
        db.parent.mkdir(parents=True, exist_ok=True)
        run([
            "sqlite3",
            str(db),
            """
            CREATE TABLE Activity (
              Id TEXT,
              Enabled BIT default TRUE,
              Type TEXT,
              Action INTEGER,
              Date TEXT,
              Data BLOB,
              PRIMARY KEY(Id, Type)
            );
            CREATE TABLE KoboPlusAssetGroup (
              Id TEXT NOT NULL,
              AssetGroup TEXT,
              Timestamp TEXT,
              Name TEXT,
              Url TEXT,
              Etag TEXT,
              TimestampTo TEXT,
              Shown BOOL DEFAULT FALSE,
              PRIMARY KEY (Id)
            );
            CREATE TABLE KoboPlusAssets (
              AssetGroupId TEXT NOT NULL,
              Key TEXT NOT NULL,
              Language TEXT NOT NULL,
              Type TEXT,
              Value TEXT,
              FOREIGN KEY (AssetGroupId) REFERENCES KoboPlusAssetGroup(Id),
              PRIMARY KEY (AssetGroupId, Key, Language)
            );
            CREATE TABLE SubscriptionProducts (
              CrossRevisionId TEXT NOT NULL,
              Id TEXT NOT NULL,
              Name TEXT,
              IsPreOrder BOOL,
              Tiers TEXT,
              ActivationDate TEXT,
              DeactivationDate TEXT,
              PRIMARY KEY (CrossRevisionId)
            );
            CREATE TABLE user (
              UserID TEXT NOT NULL,
              Subscription INT NOT NULL DEFAULT 0,
              NewUserPromoCurrency TEXT,
              NewUserPromoValue REAL NOT NULL DEFAULT -1.0,
              PRIMARY KEY (UserID)
            );
            INSERT INTO Activity VALUES
              ('store', 'true', 'Bookstore', 2, '2026-01-01T00:00:00', NULL),
              ('recent', 'true', 'RecentBook', 2, '2026-01-01T00:00:00', NULL);
            INSERT INTO KoboPlusAssetGroup VALUES
              ('group', 'EPD-KoboPlus-ReadOnly-NeverSubscribed', '2026-01-01', 'name', 'url', 'etag', NULL, true);
            INSERT INTO KoboPlusAssets VALUES
              ('group', 'Faq_CheckoutCTA', 'it', 'Text', 'Inizia la prova gratuita');
            INSERT INTO SubscriptionProducts VALUES
              ('cross', 'product', 'Kobo Plus', false, '{}', NULL, NULL);
            INSERT INTO user VALUES
              ('user', 1, 'EUR', 5.0);
            """,
        ])

        result = run(["sh", "scripts/disable-mounted-kobo-store-promos.sh", str(volume)])
        assert "Cleared Kobo Plus promo groups: 1 -> 0." in result.stdout
        assert "Cleared Kobo Plus promo assets: 1 -> 0." in result.stdout
        assert "Cleared subscription products: 1 -> 0." in result.stdout

        conf_text = conf.read_text()
        assert "KoboPlusPromoShown=true" in conf_text
        assert "kobo_subscriptions_enabled=False" in conf_text
        assert "featured_lists=\n" in conf_text
        assert "store_home=\n" in conf_text
        assert "subs_landing_page=\n" in conf_text
        assert "user_recommendations=\n" in conf_text

        active_store = run([
            "sqlite3",
            str(db),
            "SELECT count(*) FROM Activity WHERE Enabled = 'true' AND Type = 'Bookstore';",
        ]).stdout.strip()
        active_recent = run([
            "sqlite3",
            str(db),
            "SELECT count(*) FROM Activity WHERE Enabled = 'true' AND Type = 'RecentBook';",
        ]).stdout.strip()
        subscription_state = run([
            "sqlite3",
            str(db),
            "SELECT Subscription, NewUserPromoCurrency, NewUserPromoValue FROM user;",
        ]).stdout.strip()
        assert active_store == "0"
        assert active_recent == "1"
        assert subscription_state == "0||0.0"


def test_home_cleanup_install_helper_stages_patch() -> None:
    patch = ROOT / "dist" / "KoboRoot.home-cleanup.tgz"
    restore = ROOT / "dist" / "KoboRoot.home-cleanup-restore.tgz"
    if not patch.exists() or not restore.exists():
        return

    with tempfile.TemporaryDirectory() as td:
        tmp = Path(td)
        volume = tmp / "KOBOeReader"
        kobo_dir = volume / ".kobo"
        kobo_dir.mkdir(parents=True)
        (kobo_dir / "version").write_text(
            "N70977L010117,3.0.35+,4.38.23697,3.0.35+,3.0.35+,00000000-0000-0000-0000-000000000373\n"
        )

        result = run(["sh", "scripts/install-home-cleanup-to-mounted-kobo.sh", str(volume)])
        assert "Copied home-cleanup patch" in result.stdout
        assert (kobo_dir / "KoboRoot.tgz").exists()
        assert (volume / ".adds" / "matter" / "KoboRoot.home-cleanup-restore.tgz").exists()
        names = tar_names(kobo_dir / "KoboRoot.tgz")
        assert "usr/local/Kobo/libnickel.so.1.0.0" in names
        assert "usr/local/Kobo/nickel" in names


def test_home_cleanup_install_helper_rejects_wrong_firmware() -> None:
    patch = ROOT / "dist" / "KoboRoot.home-cleanup.tgz"
    restore = ROOT / "dist" / "KoboRoot.home-cleanup-restore.tgz"
    if not patch.exists() or not restore.exists():
        return

    with tempfile.TemporaryDirectory() as td:
        tmp = Path(td)
        volume = tmp / "KOBOeReader"
        kobo_dir = volume / ".kobo"
        kobo_dir.mkdir(parents=True)
        (kobo_dir / "version").write_text(
            "N70977L010117,3.0.35+,4.39.22801,3.0.35+,3.0.35+,00000000-0000-0000-0000-000000000373\n"
        )

        result = subprocess.run(
            ["sh", "scripts/install-home-cleanup-to-mounted-kobo.sh", str(volume)],
            cwd=ROOT,
            text=True,
            capture_output=True,
        )
        assert result.returncode != 0
        assert "only verified for firmware 4.38.23697" in result.stdout
        assert not (kobo_dir / "KoboRoot.tgz").exists()


def test_home_cleanup_restore_helper_stages_restore() -> None:
    restore = ROOT / "dist" / "KoboRoot.home-cleanup-restore.tgz"
    if not restore.exists():
        return

    with tempfile.TemporaryDirectory() as td:
        tmp = Path(td)
        volume = tmp / "KOBOeReader"
        kobo_dir = volume / ".kobo"
        kobo_dir.mkdir(parents=True)
        (kobo_dir / "version").write_text(
            "N70977L010117,3.0.35+,4.38.23697,3.0.35+,3.0.35+,00000000-0000-0000-0000-000000000373\n"
        )

        result = run(["sh", "scripts/install-home-cleanup-restore-to-mounted-kobo.sh", str(volume)])
        assert "Copied home-cleanup restore package" in result.stdout
        names = tar_names(kobo_dir / "KoboRoot.tgz")
        assert "usr/local/Kobo/libnickel.so.1.0.0" in names
        assert "usr/local/Kobo/nickel" in names


def test_home_cleanup_verify_helper_reports_consumed_update() -> None:
    restore = ROOT / "dist" / "KoboRoot.home-cleanup-restore.tgz"
    if not restore.exists():
        return

    with tempfile.TemporaryDirectory() as td:
        tmp = Path(td)
        volume = tmp / "KOBOeReader"
        kobo_dir = volume / ".kobo"
        restore_dir = volume / ".adds" / "matter"
        restore_dir.mkdir(parents=True)
        kobo_dir.mkdir(parents=True)
        (kobo_dir / "version").write_text(
            "N70977L010117,3.0.35+,4.38.23697,3.0.35+,3.0.35+,00000000-0000-0000-0000-000000000373\n"
        )
        shutil.copy2(restore, restore_dir / "KoboRoot.home-cleanup-restore.tgz")

        result = run(["sh", "scripts/verify-mounted-home-cleanup.sh", str(volume)])
        assert "Home-cleanup updater was consumed" in result.stdout
        assert "Matching restore package is present" in result.stdout


def test_home_cleanup_verify_helper_reports_pending_update() -> None:
    patch = ROOT / "dist" / "KoboRoot.home-cleanup.tgz"
    restore = ROOT / "dist" / "KoboRoot.home-cleanup-restore.tgz"
    if not patch.exists() or not restore.exists():
        return

    with tempfile.TemporaryDirectory() as td:
        tmp = Path(td)
        volume = tmp / "KOBOeReader"
        kobo_dir = volume / ".kobo"
        restore_dir = volume / ".adds" / "matter"
        restore_dir.mkdir(parents=True)
        kobo_dir.mkdir(parents=True)
        (kobo_dir / "version").write_text(
            "N70977L010117,3.0.35+,4.38.23697,3.0.35+,3.0.35+,00000000-0000-0000-0000-000000000373\n"
        )
        shutil.copy2(patch, kobo_dir / "KoboRoot.tgz")
        shutil.copy2(restore, restore_dir / "KoboRoot.home-cleanup-restore.tgz")

        result = subprocess.run(
            ["sh", "scripts/verify-mounted-home-cleanup.sh", str(volume)],
            cwd=ROOT,
            text=True,
            capture_output=True,
        )
        assert result.returncode == 2
        assert "Home-cleanup update is still pending" in result.stdout


def test_label_helper_creates_matter_collection() -> None:
    if shutil.which("sqlite3") is None:
        return
    with tempfile.TemporaryDirectory() as td:
        tmp = Path(td)
        volume = tmp / "KOBOeReader"
        db = volume / ".kobo" / "KoboReader.sqlite"
        db.parent.mkdir(parents=True)
        create_minimal_kobo_db(db)

        run(["sh", "scripts/label-mounted-kobo-matter.sh", str(volume)])
        count = run([
            "sqlite3",
            str(db),
            "SELECT count(*) FROM ShelfContent WHERE ShelfName = 'Matter' AND _IsDeleted = 'false';",
        ]).stdout.strip()
        assert count == "1"
        backup_dir = volume / ".adds" / "matter" / "db-backups"
        assert any(backup_dir.glob("KoboReader.before-matter-collection.*.sqlite"))


def main() -> None:
    test_combine_koboroot()
    test_install_helper_preserves_existing_update()
    test_install_helper_seeds_token()
    test_update_helper_preserves_config()
    test_finalize_helper_runs_post_sync_pass()
    test_clean_home_helper_disables_store_widgets()
    test_store_promo_helper_disables_kobo_plus_sources()
    test_home_cleanup_install_helper_stages_patch()
    test_home_cleanup_install_helper_rejects_wrong_firmware()
    test_home_cleanup_restore_helper_stages_restore()
    test_home_cleanup_verify_helper_reports_consumed_update()
    test_home_cleanup_verify_helper_reports_pending_update()
    test_label_helper_creates_matter_collection()
    print("test-install-tools: ok")


if __name__ == "__main__":
    main()
