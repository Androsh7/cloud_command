"""Defines the CloudCommand API"""

# Standard libraries
from contextlib import asynccontextmanager

# Third-party libraries
from fastapi import FastAPI

from cloud_command.agents.agent_manager import agent_manager

# Project libraries
from cloud_command.constants import VERSION
from cloud_command.router.agent import cluster_router


@asynccontextmanager
async def lifespan(app: FastAPI):
    # Startup actions
    agent_manager.load_all_agents()

    yield

    # Shutdown actions


app = FastAPI(title="CloudCommand", docs_url="/api/docs", version=VERSION, lifespan=lifespan)
app.include_router(cluster_router, prefix="/api")
