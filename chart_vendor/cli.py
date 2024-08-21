# Copyright (c) 2024 VEXXHOST, Inc.
# SPDX-License-Identifier: Apache-2.0

import argparse
import asyncio
import os
import textwrap
from datetime import datetime, timezone

import aiohttp
import aiohttp_client_cache
import aiohttp_retry
import aiopath  # type: ignore
import aioshutil
from asynctempfile import NamedTemporaryFile  # type: ignore
from gerrit import GerritClient  # type: ignore
from git import GitCommandError, Repo
from loguru import logger
from pydantic import BaseModel
from pydantic_yaml import parse_yaml_file_as, to_yaml_file

from chart_vendor import models, parsers


async def patch(input: bytes, path: aiopath.AsyncPath):
    async with NamedTemporaryFile() as temp_file:
        await temp_file.write(
            textwrap.dedent(
                f"""\
                {path.name}/*
                """
            )
            .strip()
            .encode()
        )
        await temp_file.flush()

        proc = await asyncio.create_subprocess_shell(
            f"filterdiff -p1 -I {temp_file.name}",
            stdin=asyncio.subprocess.PIPE,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        stdout, stderr = await proc.communicate(input=input)
        if proc.returncode != 0:
            raise Exception(stderr)

    async with NamedTemporaryFile() as temp_file:
        await temp_file.write(
            textwrap.dedent(
                f"""\
                {path.name}/Chart.yaml
                {path.name}/values_overrides/*
                """
            )
            .strip()
            .encode()
        )
        await temp_file.flush()

        proc = await asyncio.create_subprocess_shell(
            f"filterdiff -p1 -X {temp_file.name}",
            stdin=asyncio.subprocess.PIPE,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        stdout, stderr = await proc.communicate(input=stdout)
        if proc.returncode != 0:
            raise Exception(stderr)

    proc = await asyncio.create_subprocess_shell(
        f"patch -p2 -d {path} -E",
        stdin=asyncio.subprocess.PIPE,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE,
    )
    stdout, stderr = await proc.communicate(input=stdout)
    if proc.returncode != 0:
        raise Exception(stdout)


class Config(BaseModel):
    charts: list[models.Chart]

    @property
    def repositories(self):
        repositories = []

        for chart in self.charts:
            if chart.repository in repositories:
                continue
            repositories.append(chart.repository)

        return repositories

    async def _fetch_chart(
        self,
        session: aiohttp_client_cache.CachedSession,
        chart: models.Chart,
        path: str,
    ):
        charts_path: aiopath.AsyncPath = aiopath.AsyncPath(path)
        chart_path = charts_path / chart.name

        try:
            await aioshutil.rmtree(f"{path}/{chart.name}-{chart.version}")
        except FileNotFoundError:
            pass

        try:
            try:
                os.rename(
                    f"{path}/{chart.name}", f"{path}/{chart.name}-{chart.version}"
                )
            except FileNotFoundError:
                pass

            await parsers.fetch_chart(
                session, str(chart.repository.url), chart.name, chart.version, path
            )
        except Exception:
            os.rename(f"{path}/{chart.name}-{chart.version}", f"{path}/{chart.name}")
            raise

        try:
            await aioshutil.rmtree(f"{path}/{chart.name}-{chart.version}")
        except FileNotFoundError:
            pass

        if chart.dependencies:
            requirements = models.ChartRequirements(dependencies=chart.dependencies)
            to_yaml_file(f"{path}/{chart.name}/requirements.yaml", requirements)

            await asyncio.gather(
                *[
                    aioshutil.rmtree(f"{path}/{chart.name}/charts/{req.name}")
                    for req in chart.dependencies
                ]
            )

            await asyncio.gather(
                *[
                    parsers.fetch_chart(
                        session,
                        str(req.repository),
                        req.name,
                        req.version,
                        f"{path}/{chart.name}/charts",
                    )
                    for req in chart.dependencies
                ]
            )

            for req in chart.dependencies:
                lock = parse_yaml_file_as(
                    models.ChartLock,
                    f"{path}/{chart.name}/charts/{req.name}/requirements.lock",
                )
                lock.generated = datetime.min.replace(tzinfo=timezone.utc)
                to_yaml_file(
                    f"{path}/{chart.name}/charts/{req.name}/requirements.lock", lock
                )

            # Reset the generated time in the lock file to make things reproducible
            to_yaml_file(
                f"{path}/{chart.name}/requirements.lock", chart.requirements_lock
            )

        for gerrit, changes in chart.patches.gerrit.items():
            client = GerritClient(base_url=f"https://{gerrit}")

            for change_id in changes:
                change = client.changes.get(change_id)
                gerrit_patch = change.get_revision().get_patch(decode=True)
                await patch(input=gerrit_patch.encode(), path=chart_path)

        patches_path = charts_path / "patches" / chart.name

        if await patches_path.exists():
            patch_paths = sorted(
                [patch_path async for patch_path in patches_path.glob("*.patch")]
            )
            for patch_path in patch_paths:
                async with patch_path.open(mode="rb") as patch_file:
                    patch_data = await patch_file.read()
                    await patch(input=patch_data, path=chart_path)


async def _main(
    config_file: str, charts_root: str, check: bool, chart_name: str = None
):
    config = parse_yaml_file_as(Config, config_file)

    async with aiohttp_retry.RetryClient(
        client_session=aiohttp_client_cache.CachedSession(
            connector=aiohttp.TCPConnector(limit_per_host=5),
            cache=aiohttp_client_cache.FileBackend(use_temp=True),
        ),
        retry_options=aiohttp_retry.ExponentialRetry(attempts=3),
    ) as session:
        charts_to_fetch = (
            [chart for chart in config.charts if chart.name == chart_name]
            if chart_name
            else config.charts
        )
        if not charts_to_fetch:
            logger.warn("No chart configured to fetch.")
            return
        await asyncio.gather(
            *[
                config._fetch_chart(session, chart, path=charts_root)
                for chart in charts_to_fetch
            ]
        )

        if check:
            repo = Repo(os.getcwd())
            changed_files = [item.a_path for item in repo.index.diff(None)]
            untracked_files = repo.untracked_files

            if changed_files or untracked_files:
                logger.info(
                    "The following chart manifests have changes or are untracked:"
                )
                for file in changed_files:
                    logger.info(f"Modified: {file}")
                for file in untracked_files:
                    logger.info(f"Untracked: {file}")

                try:
                    diff_output = repo.git.diff()
                    if diff_output:
                        logger.info("Diff output:")
                        logger.info(diff_output)
                except GitCommandError as e:
                    logger.error(f"Failed to get git diff: {e}")

                raise SystemExit(
                    "Check failed: Uncommitted changes or untracked files found in chart manifests."
                )


def main():
    parser = argparse.ArgumentParser(description="Chart Vendor CLI")
    parser.add_argument(
        "chart_name",
        type=str,
        nargs="?",
        help="Name of the specific chart to fetch",
    )
    parser.add_argument(
        "--config-file",
        type=str,
        default=".charts.yml",
        help="Configuration file for the vendored charts",
    )
    parser.add_argument(
        "--charts-root",
        type=str,
        default="charts",
        help="Root path where charts are generated",
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="Check if all chart manifests are applied or not",
    )
    args = parser.parse_args()

    asyncio.run(_main(args.config_file, args.charts_root, args.check, args.chart_name))


if __name__ == "__main__":
    main()
