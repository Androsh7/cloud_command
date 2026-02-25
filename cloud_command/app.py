"""Defines the CloudCommand API"""

# Standard libraries
from contextlib import asynccontextmanager

# Third-party libraries
from fastapi import FastAPI, Response
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles
from loguru import logger

# Project libraries
from cloud_command.agent.agent_manager import agent_manager
from cloud_command.constants import REACT_FILE_PATH, VERSION
from cloud_command.router.agent import cluster_router
from cloud_command.router.command import command_router


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
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


class SPAFileServer(StaticFiles):
    def __init__(self, directory: str | None = None, html: bool = False, check_dir: bool = True) -> None:
        super().__init__(directory=directory, html=html, check_dir=check_dir)

    async def get_response(self, path: str, scope: dict) -> Response:
        try:
            return await super().get_response(path, scope)
        except Exception:
            logger.debug(f'SPA Redirect: "/{path}" -> "/index.html{scope["path"]}"')
            return await super().get_response("index.html", scope)


app.mount("/", SPAFileServer(directory=REACT_FILE_PATH, html=True), name="react-app")
