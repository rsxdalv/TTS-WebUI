"""The history tabs pass a client-chosen folder to the desktop opener."""

import os
import sys

import pytest

from tts_webui.utils import open_folder as of


@pytest.fixture
def opened(monkeypatch):
    calls = []
    monkeypatch.setattr(of, "_open_in_file_manager", calls.append)
    return calls


@pytest.fixture
def root(tmp_path):
    for d in ("outputs/gen1", "favorites/gen2", "collections/mine/gen3", "data"):
        (tmp_path / d).mkdir(parents=True)
    (tmp_path / "outputs" / "gen1" / "a.wav").write_bytes(b"x")
    (tmp_path / "secret.txt").write_text("s")
    return str(tmp_path)


@pytest.mark.parametrize(
    "rel",
    ["outputs", "outputs/gen1", "favorites/gen2", "collections/mine", "collections/mine/gen3"],
)
def test_collection_folders_open(root, opened, rel):
    of.open_collection_folder(rel, root)
    assert opened == [os.path.realpath(os.path.join(root, *rel.split("/")))]


def test_backslashes_from_gradio_on_windows_are_accepted(root, opened):
    of.open_collection_folder("collections\\mine\\gen3", root)
    assert len(opened) == 1


@pytest.mark.parametrize(
    "rel",
    [
        "",
        "../",
        "outputs/../data",
        "outputs/../../etc",
        "/etc",
        "/etc/passwd",
        "C:/Windows",
        "C:\\Windows\\System32\\calc.exe",
        "data",  # not a collection root
        "collections",  # needs a name
        "outputs//gen1",
        "outputs/.hidden",
        "outputs/gen1\x00",
        "outputs/gen1/a.wav",  # a file, never handed to the opener
        "outputs/missing",
    ],
)
def test_everything_else_is_refused(root, opened, rel):
    with pytest.raises(ValueError):
        of.open_collection_folder(rel, root)
    assert opened == []


def test_non_strings_are_refused(root, opened):
    for bad in (None, 5, ["outputs"]):
        with pytest.raises(ValueError):
            of.open_collection_folder(bad, root)
    assert opened == []


@pytest.mark.skipif(sys.platform == "win32", reason="symlinks need privileges on Windows")
def test_a_symlink_out_of_a_collection_is_refused(root, opened, tmp_path_factory):
    outside = tmp_path_factory.mktemp("outside")
    os.symlink(outside, os.path.join(root, "outputs", "escape"))
    with pytest.raises(ValueError, match="leaves the collection"):
        of.open_collection_folder("outputs/escape", root)
    assert opened == []


@pytest.mark.skipif(sys.platform == "win32", reason="symlinks need privileges on Windows")
def test_a_collection_root_may_itself_be_a_link(root, opened, tmp_path_factory):
    # outputs/ on another disk is a supported setup.
    other = tmp_path_factory.mktemp("disk2")
    (other / "gen9").mkdir()
    os.rmdir(os.path.join(root, "favorites", "gen2"))
    os.rmdir(os.path.join(root, "favorites"))
    os.symlink(other, os.path.join(root, "favorites"))
    of.open_collection_folder("favorites/gen9", root)
    assert opened == [os.path.realpath(other / "gen9")]


def test_open_folder_needs_an_existing_directory(root, opened):
    of.open_folder(os.path.join(root, "data"))
    assert opened == [os.path.realpath(os.path.join(root, "data"))]
    for bad in ("", os.path.join(root, "secret.txt"), os.path.join(root, "nope"), None):
        with pytest.raises(ValueError):
            of.open_folder(bad)
    assert len(opened) == 1
