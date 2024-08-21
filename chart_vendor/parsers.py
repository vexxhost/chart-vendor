# Copyright (c) 2024 VEXXHOST, Inc.
# SPDX-License-Identifier: Apache-2.0

import io
import tarfile

import aiohttp_client_cache
import yaml  # type: ignore
from async_lru import alru_cache
from loguru import logger
from tenacity import (
    retry,
    retry_if_exception_type,
    stop_after_attempt,
    wait_random_exponential,
)


@alru_cache(maxsize=32)
async def parse_remote_repository(
    session: aiohttp_client_cache.CachedSession, url: str
):
    repo_url = str(url) + "/index.yaml"

    async with session.get(repo_url) as resp:
        data = await resp.text()
        return yaml.safe_load(data)


def fetch_entry(index: dict, index_url: str, name: str, version: str):
    logger.info(f"Looking up {name} with version {version}")

    entries = index["entries"].get(name, [])
    entry = next(
        (entry for entry in entries if entry["version"].replace("v", "") == version),
        None,
    )

    if not entry:
        raise Exception(f"Could not find entry for {name} with version {version}")

    # NOTE(mnaser): Some repositories have the URL as a relative path
    #               so we need to make sure we have the full URL.
    for idx, url in enumerate(entry["urls"]):
        if url.startswith("http"):
            break
        entry["urls"][idx] = index_url + "/" + url

    logger.info(f"Found entry {entry}")

    return entry


@retry(
    retry=retry_if_exception_type(ConnectionResetError),
    wait=wait_random_exponential(multiplier=1, min=2, max=10),
    stop=stop_after_attempt(10),
)
async def fetch_chart(
    session: aiohttp_client_cache.CachedSession,
    index_url: str,
    name: str,
    version: str,
    path: str,
):
    index = await parse_remote_repository(session, index_url)
    entry = fetch_entry(index, index_url, name, version)

    url = entry["urls"][0]
    logger.info(f"Fetching {name} from {url}")

    async with session.get(url) as resp:
        data = await resp.read()

    tar_bytes = io.BytesIO(data)
    with tarfile.open(fileobj=tar_bytes) as tar:
        tar.extractall(path=path)
