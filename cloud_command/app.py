"""Defines the CloudCommand API"""

# Standard libraries
from contextlib import asynccontextmanager

# Third-party libraries
from fastapi import FastAPI, Response

from cloud_command.agent.agent_manager import agent_manager

# Project libraries
from cloud_command.constants import VERSION, REACT_FILE_PATH
from cloud_command.router.agent import cluster_router
from cloud_command.router.command import command_router
from fastapi.staticfiles import StaticFiles


@asynccontextmanager
async def lifespan(app: FastAPI):
    # Startup actions
    agent_manager.load_all_agents()

    yield

    # Shutdown actions


app = FastAPI(
    title="CloudCommand", docs_url="/api/docs", openapi_url="/api/openapi.json", version=VERSION, lifespan=lifespan
)
app.include_router(cluster_router, prefix="/api")
app.include_router(command_router, prefix="/api")

class SPAFileServer(StaticFiles):
    def __init__(self, directory: str | None = None, html: bool = False, check_dir: bool = True) -> None:
        super().__init__(directory=directory, html=html, check_dir=check_dir)

    async def get_response(self, path: str, scope: dict) -> Response:
        try:
            return await super().get_response(path, scope)
        except Exception:
            return await super().get_response("index.html", scope)

app.mount("/", SPAFileServer(directory=REACT_FILE_PATH, html=True), name="react-app")