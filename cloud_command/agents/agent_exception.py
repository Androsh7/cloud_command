"""Defines AgentException classes"""


class AgentException(Exception):
    pass


class AgentBuildException(AgentException):
    pass


class AgentRuntimeException(AgentException):
    pass
