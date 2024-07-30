# Copyright (c) 2024 VEXXHOST, Inc.
# SPDX-License-Identifier: Apache-2.0

import hashlib
import json
from datetime import datetime, timezone
from typing import Annotated

from pydantic import AfterValidator, BaseModel, HttpUrl
from pydantic.json import pydantic_encoder

HttpUrlString = Annotated[HttpUrl, AfterValidator(str)]


class ChartRepository(BaseModel):
    url: HttpUrlString

    @property
    def name(self):
        return self.url.host.replace(".", "-") + self.url.path.replace("/", "-")


class ChartPatches(BaseModel):
    gerrit: dict[str, list[int]] = {}


class ChartDependency(BaseModel):
    name: str
    version: str
    repository: HttpUrlString


class ChartRequirements(BaseModel):
    dependencies: list[ChartDependency] = []


class ChartLock(BaseModel):
    dependencies: list[ChartDependency] = []
    digest: str
    generated: datetime

    class Config:
        json_encoders = {
            "generated": lambda dt: dt.isoformat(),
        }


class Chart(BaseModel):
    name: str
    version: str
    repository: ChartRepository
    dependencies: list[ChartDependency] = []
    patches: ChartPatches = ChartPatches()

    @property
    def requirements_lock(self):
        data = json.dumps(
            [
                self.dependencies,
                self.dependencies,
            ],
            default=pydantic_encoder,
            separators=(",", ":"),
        ).encode("utf-8")

        return ChartLock(
            dependencies=self.dependencies,
            digest="sha256:" + hashlib.sha256(data).hexdigest(),
            generated=datetime.min.replace(tzinfo=timezone.utc),
        )
