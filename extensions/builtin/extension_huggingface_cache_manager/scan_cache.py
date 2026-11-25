# coding=utf-8
# Copyright 2022-present, the HuggingFace Inc. team.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
"""Contains command to scan the HF cache directory.

Usage:
    huggingface-cli scan-cache
    huggingface-cli scan-cache -v
    huggingface-cli scan-cache -vvv
    huggingface-cli scan-cache --dir ~/.cache/huggingface/hub
"""
import time
from typing import Optional

from huggingface_hub.utils import CacheNotFound, HFCacheInfo, scan_cache_dir
from pathlib import Path

try:
    from huggingface_hub.commands._cli_utils import ANSI, tabulate
except ImportError:
    # huggingface_hub 1.x removed the `commands` package, so keep our own copy
    # of the two small helpers (from huggingface_hub 0.x commands/_cli_utils.py).
    class ANSI:
        _bold = "\u001b[1m"
        _gray = "\u001b[90m"
        _red = "\u001b[31m"
        _reset = "\u001b[0m"

        @classmethod
        def bold(cls, s: str) -> str:
            return cls._format(s, cls._bold)

        @classmethod
        def gray(cls, s: str) -> str:
            return cls._format(s, cls._gray)

        @classmethod
        def red(cls, s: str) -> str:
            return cls._format(s, cls._bold + cls._red)

        @classmethod
        def _format(cls, s: str, code: str) -> str:
            return f"{code}{s}{cls._reset}"

    def tabulate(rows: list[list[str | int]], headers: list[str]) -> str:
        """Render rows as a plain-text table with padded columns."""
        col_widths = [max(len(str(x)) for x in col) for col in zip(*rows, headers)]
        row_format = ("{{:{}}} " * len(headers)).format(*col_widths)
        lines = [row_format.format(*headers)]
        lines.append(row_format.format(*["-" * w for w in col_widths]))
        for row in rows:
            lines.append(row_format.format(*row))
        return "\n".join(lines)

def get_rows_json(hf_cache_info: HFCacheInfo) -> list[list[str | int | float]]:
    return [
        [
            repo.repo_id,
            repo.repo_type,
            revision.commit_hash,
            "{:>12}".format(revision.size_on_disk_str),
            repo.size_on_disk,
            revision.size_on_disk,
            revision.nb_files,
            repo.last_accessed_str,
            repo.last_accessed,
            revision.last_modified_str,
            revision.last_modified,
            ", ".join(sorted(revision.refs)),
            str(revision.snapshot_path),
        ]
        for repo in sorted(hf_cache_info.repos, key=lambda repo: repo.repo_path)
        for revision in sorted(
            repo.revisions, key=lambda revision: revision.commit_hash
        )
    ]

def get_headers_json() -> list[str]:
    return [
        "repo_id",
        "repo_type",
        "commit_hash",
        "size_on_disk_str",
        "repo_size_on_disk",
        "revision_size_on_disk",
        "nb_files",
        "last_accessed_str",
        "last_accessed",
        "last_modified_str",
        "last_modified",
        "refs",
        "snapshot_path",
    ]


def get_rows_verbose(hf_cache_info: HFCacheInfo) -> list[list[str | int]]:
    return [
        [
            repo.repo_id,
            repo.repo_type,
            revision.commit_hash,
            "{:>12}".format(revision.size_on_disk_str),
            revision.nb_files,
            revision.last_modified_str,
            ", ".join(sorted(revision.refs)),
            str(revision.snapshot_path),
        ]
        for repo in sorted(hf_cache_info.repos, key=lambda repo: repo.repo_path)
        for revision in sorted(
            repo.revisions, key=lambda revision: revision.commit_hash
        )
    ]


def get_rows_quiet(hf_cache_info: HFCacheInfo) -> list[list[str | int]]:
    return [
        [
            repo.repo_id,
            repo.repo_type,
            "{:>12}".format(repo.size_on_disk_str),
            repo.nb_files,
            repo.last_accessed_str,
            repo.last_modified_str,
            ", ".join(sorted(repo.refs)),
            str(repo.repo_path),
        ]
        for repo in sorted(hf_cache_info.repos, key=lambda repo: repo.repo_path)
    ]


def get_rows(verbosity: int, hf_cache_info: HFCacheInfo) -> list[list[str | int]]:
    if verbosity == 0:
        return get_rows_quiet(hf_cache_info)
    else:
        return get_rows_verbose(hf_cache_info)


def get_headers_verbose() -> list[str]:
    return [
        "REPO ID",
        "REPO TYPE",
        "REVISION",
        "SIZE ON DISK",
        "NB FILES",
        "LAST_MODIFIED",
        "REFS",
        "LOCAL PATH",
    ]


def get_headers_quiet() -> list[str]:
    return [
        "REPO ID",
        "REPO TYPE",
        "SIZE ON DISK",
        "NB FILES",
        "LAST_ACCESSED",
        "LAST_MODIFIED",
        "REFS",
        "LOCAL PATH",
    ]


def render_as_markdown(rows: list[list[str | int]], headers: list[str]) -> str:
    markdown = ""
    # render headers
    markdown += " | ".join(headers) + "\n"
    markdown += " | ".join(["---"] * len(headers)) + "\n"
    # render rows
    for row in rows:
        markdown += " | ".join([str(x) for x in row]) + "\n"
    return markdown


def get_table(verbosity: int, hf_cache_info: HFCacheInfo) -> str:
    if verbosity == 0:
        return tabulate(
            rows=[
                [
                    repo.repo_id,
                    repo.repo_type,
                    "{:>12}".format(repo.size_on_disk_str),
                    repo.nb_files,
                    repo.last_accessed_str,
                    repo.last_modified_str,
                    ", ".join(sorted(repo.refs)),
                    str(repo.repo_path),
                ]
                for repo in sorted(hf_cache_info.repos, key=lambda repo: repo.repo_path)
            ],
            headers=[
                "REPO ID",
                "REPO TYPE",
                "SIZE ON DISK",
                "NB FILES",
                "LAST_ACCESSED",
                "LAST_MODIFIED",
                "REFS",
                "LOCAL PATH",
            ],
        )
    else:
        return tabulate(
            rows=[
                [
                    repo.repo_id,
                    repo.repo_type,
                    revision.commit_hash,
                    "{:>12}".format(revision.size_on_disk_str),
                    revision.nb_files,
                    revision.last_modified_str,
                    ", ".join(sorted(revision.refs)),
                    str(revision.snapshot_path),
                ]
                for repo in sorted(hf_cache_info.repos, key=lambda repo: repo.repo_path)
                for revision in sorted(
                    repo.revisions, key=lambda revision: revision.commit_hash
                )
            ],
            headers=[
                "REPO ID",
                "REPO TYPE",
                "REVISION",
                "SIZE ON DISK",
                "NB FILES",
                "LAST_MODIFIED",
                "REFS",
                "LOCAL PATH",
            ],
        )


def scan_cache_and_print(
    verbosity: int = 0, cache_dir: Optional[str | Path] = None
) -> None:
    try:
        t0 = time.time()
        hf_cache_info = scan_cache_dir(cache_dir)
        t1 = time.time()
    except CacheNotFound as exc:
        cache_dir = exc.cache_dir
        print(f"Cache directory not found: {cache_dir}")
        return

    print(get_table(verbosity, hf_cache_info))

    print(
        f"\nDone in {round(t1-t0,1)}s. Scanned {len(hf_cache_info.repos)} repo(s)"
        f" for a total of {ANSI.red(hf_cache_info.size_on_disk_str)}."
    )
    if len(hf_cache_info.warnings) > 0:
        message = f"Got {len(hf_cache_info.warnings)} warning(s) while scanning."
        if verbosity >= 3:
            print(ANSI.gray(message))
            for warning in hf_cache_info.warnings:
                print(ANSI.gray(warning))
        else:
            print(ANSI.gray(message + " Use -vvv to print details."))
