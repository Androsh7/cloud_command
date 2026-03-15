"""Defines the CloudCommand API"""

# Standard libraries
import asyncio
import sys
from contextlib import asynccontextmanager

# Third-party libraries
from fastapi import FastAPI, Request, Response
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
from fastapi.staticfiles import StaticFiles
from loguru import logger

# Project libraries
from cloud_command.agent.agent_manager import agent_manager
from cloud_command.constants import REACT_FILE_PATH, STATUS_TRACKER_UPDATE_INTERVAL, VERSION
from cloud_command.mcp_server import mcp_http_app
from cloud_command.router.agent import cluster_router
from cloud_command.router.command import command_router
from cloud_command.router.error_model import ServerError
from cloud_command.router.shell import shell_router


async def status_update_loop():
    while True:
        try:
            if not len(agent_manager.agent_list) == 0:
                logger.debug(f"Updating agent statuses ({STATUS_TRACKER_UPDATE_INTERVAL}s interval)")
                await agent_manager.update_all()
        except Exception as ex:
            logger.error(f"Failed to update agent statuses: {ex}")
        await asyncio.sleep(STATUS_TRACKER_UPDATE_INTERVAL)


def _windows_connection_reset_handler(loop, context):
    """Suppress WinError 10054 from SSH connection teardown on Windows"""
    if sys.platform == "win32" and isinstance(context.get("exception"), ConnectionResetError):
        return
    loop.default_exception_handler(context)


@asynccontextmanager
async def lifespan(app: FastAPI):
    async with mcp_http_app.router.lifespan_context(app):
        if sys.platform == "win32":
            asyncio.get_event_loop().set_exception_handler(_windows_connection_reset_handler)
        agent_manager.load_all_agents()
        status_task = asyncio.create_task(status_update_loop())
        yield
        status_task.cancel()


app = FastAPI(
    title="CloudCommand", docs_url="/api/docs", openapi_url="/api/openapi.json", version=VERSION, lifespan=lifespan
)
app.include_router(cluster_router, prefix="/api")
app.include_router(command_router, prefix="/api")
app.include_router(shell_router, prefix="/api")
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


# Add error handlers
@app.exception_handler(ServerError)
async def http_exception_handler(request: Request, exc: ServerError):
    return JSONResponse(
        content={
            "error": exc.error,
            "details": exc.details,
        },
        status_code=exc.status_code,
    )


class SPAFileServer(StaticFiles):
    def __init__(self, directory: str | None = None, html: bool = False, check_dir: bool = True):
        super().__init__(directory=directory, html=html, check_dir=check_dir)

    async def get_response(self, path: str, scope: dict) -> Response:
        try:
            return await super().get_response(path, scope)
        except Exception:
            logger.debug(f'SPA Redirect: "/{path}" -> "/index.html{scope["path"]}"')
            return await super().get_response("index.html", scope)


app.mount("/api/mcp", mcp_http_app)
app.mount("/", SPAFileServer(directory=REACT_FILE_PATH, html=True), name="react-app")
