"""Defines the CloudCommand API"""

# Standard libraries
from contextlib import asynccontextmanager

# Third-party libraries
from fastapi import FastAPI

from cloud_command.agent.agent_manager import agent_manager

# Project libraries
from cloud_command.constants import VERSION
from cloud_command.router.agent import cluster_router
from cloud_command.router.auth import auth_router
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
app.include_router(auth_router, prefix="/api")
app.include_router(cluster_router, prefix="/api")
app.include_router(command_router, prefix="/api")
