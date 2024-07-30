# Copyright (c) 2024 VEXXHOST, Inc.
# SPDX-License-Identifier: Apache-2.0

import asyncio
import os
import textwrap
from datetime import datetime, timezone

import aiohttp_client_cache
import aiohttp_retry
import aiopath  # type: ignore
import aioshutil
from asynctempfile import NamedTemporaryFile  # type: ignore
from gerrit import GerritClient  # type: ignore
from pydantic import BaseModel, PrivateAttr
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
        path="charts",
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


async def _main():
    config = parse_yaml_file_as(Config, ".charts.yml")

    async with aiohttp_retry.RetryClient(
        client_session=aiohttp_client_cache.CachedSession(
            cache=aiohttp_client_cache.FileBackend(use_temp=True)
        ),
        retry_options=aiohttp_retry.ExponentialRetry(attempts=3),
    ) as session:
        await asyncio.gather(
            *[config._fetch_chart(session, chart) for chart in config.charts]
        )


def main():
    asyncio.run(_main())
