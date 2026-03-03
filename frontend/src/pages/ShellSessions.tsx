import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link } from "react-router-dom";
import {
  createShellSession,
  deleteShellSession,
  getAgents,
  listShellSessions,
} from "../components/Api";

export default function ShellSessions() {
  const queryClient = useQueryClient();
  const [creatingAgentName, setCreatingAgentName] = useState<string | null>(
    null,
  );
  const [deletingShellUuid, setDeletingShellUuid] = useState<string | null>(
    null,
  );
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  const {
    data: agents,
    isLoading: isAgentsLoading,
    error: agentsError,
  } = useQuery({
    queryKey: ["agents"],
    queryFn: () => getAgents(),
    refetchInterval: 5000,
  });

  const {
    data: shellSessions,
    isLoading: isShellSessionsLoading,
    error: shellSessionsError,
  } = useQuery({
    queryKey: ["shellSessions"],
    queryFn: () => listShellSessions(),
    refetchInterval: 5000,
  });

  const createShellSessionMutation = useMutation({
    mutationFn: (agentName: string) => createShellSession(agentName),
    onSuccess: (uuid, agentName) => {
      setSuccessMessage(`Created shell session ${uuid} for ${agentName}.`);
      queryClient.invalidateQueries({ queryKey: ["shellSessions"] });
    },
    onError: () => {
      setSuccessMessage(null);
    },
    onSettled: () => {
      setCreatingAgentName(null);
    },
  });

  const deleteShellSessionMutation = useMutation({
    mutationFn: (uuid: string) => deleteShellSession(uuid),
    onSuccess: () => {
      setSuccessMessage("Shell session deleted.");
      queryClient.invalidateQueries({ queryKey: ["shellSessions"] });
    },
    onError: () => {
      setSuccessMessage(null);
    },
    onSettled: () => {
      setDeletingShellUuid(null);
    },
  });

  function handleCreateShellSession(agentName: string) {
    if (createShellSessionMutation.isPending || deleteShellSessionMutation.isPending) {
      return;
    }
    setSuccessMessage(null);
    setCreatingAgentName(agentName);
    createShellSessionMutation.mutate(agentName);
  }

  function handleDeleteShellSession(uuid: string) {
    if (deleteShellSessionMutation.isPending || createShellSessionMutation.isPending) {
      return;
    }
    setSuccessMessage(null);
    setDeletingShellUuid(uuid);
    deleteShellSessionMutation.mutate(uuid);
  }

  return (
    <div className="container mt-4">
      <h1>Shell Sessions</h1>
      <p className="text-muted">
        Create shell sessions by agent, open active sessions, or delete them.
      </p>

      <div className="card mb-4">
        <div className="card-header">Create Shell Session</div>
        <div className="card-body">
          {isAgentsLoading ? <p className="mb-0">Loading agents...</p> : null}

          {agentsError ? (
            <div className="alert alert-danger mb-0">
              Failed to fetch agents: {(agentsError as Error).message}
            </div>
          ) : null}

          {!isAgentsLoading && !agentsError && agents?.length === 0 ? (
            <p className="mb-0">No agents available. Create an agent first.</p>
          ) : null}

          {!isAgentsLoading && !agentsError && agents && agents.length > 0 ? (
            <div className="list-group">
              {agents.map((agent) => (
                <div
                  key={agent.name}
                  className="list-group-item d-flex justify-content-between align-items-center"
                >
                  <div>
                    <div className="fw-bold">{agent.name}</div>
                    <small className="text-muted">{agent.state}</small>
                  </div>
                  <button
                    type="button"
                    className="btn btn-link p-0"
                    onClick={() => handleCreateShellSession(agent.name)}
                    disabled={
                      createShellSessionMutation.isPending ||
                      deleteShellSessionMutation.isPending
                    }
                  >
                    {createShellSessionMutation.isPending &&
                    creatingAgentName === agent.name
                      ? "Creating..."
                      : "Create Shell"}
                  </button>
                </div>
              ))}
            </div>
          ) : null}
        </div>
      </div>

      <div className="card">
        <div className="card-header">Active Shell Sessions</div>
        <div className="card-body">
          {isShellSessionsLoading ? (
            <p className="mb-0">Loading shell sessions...</p>
          ) : null}

          {shellSessionsError ? (
            <div className="alert alert-danger mb-0">
              Failed to fetch shell sessions:{" "}
              {(shellSessionsError as Error).message}
            </div>
          ) : null}

          {!isShellSessionsLoading &&
          !shellSessionsError &&
          shellSessions?.length === 0 ? (
            <p className="mb-0">No shell sessions are active.</p>
          ) : null}

          {!isShellSessionsLoading &&
          !shellSessionsError &&
          shellSessions &&
          shellSessions.length > 0 ? (
            <div className="list-group">
              {shellSessions.map((shellSession) => (
                <div
                  key={shellSession.uuid}
                  className="list-group-item d-flex justify-content-between align-items-center"
                >
                  <div>
                    <div className="fw-bold">
                      <code>{shellSession.uuid}</code>
                    </div>
                    <small className="text-muted">
                      Agent: {shellSession.agent_name}
                    </small>
                  </div>
                  <div className="d-flex gap-3">
                    <Link
                      to={`/shell/${shellSession.uuid}`}
                      className="btn btn-link p-0"
                    >
                      Open
                    </Link>
                    <button
                      type="button"
                      className="btn btn-link p-0 text-danger"
                      onClick={() => handleDeleteShellSession(shellSession.uuid)}
                      disabled={
                        deleteShellSessionMutation.isPending ||
                        createShellSessionMutation.isPending
                      }
                    >
                      {deleteShellSessionMutation.isPending &&
                      deletingShellUuid === shellSession.uuid
                        ? "Deleting..."
                        : "Delete"}
                    </button>
                  </div>
                </div>
              ))}
            </div>
          ) : null}
        </div>
      </div>

      {successMessage ? (
        <div className="alert alert-success mt-3 mb-0">{successMessage}</div>
      ) : null}

      {createShellSessionMutation.error ? (
        <div className="alert alert-danger mt-3 mb-0">
          Failed to create shell session:{" "}
          {(createShellSessionMutation.error as Error).message}
        </div>
      ) : null}

      {deleteShellSessionMutation.error ? (
        <div className="alert alert-danger mt-3 mb-0">
          Failed to delete shell session:{" "}
          {(deleteShellSessionMutation.error as Error).message}
        </div>
      ) : null}
    </div>
  );
}
