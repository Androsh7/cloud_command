import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import {
  createAgent,
  createShellSession,
  deleteAgent,
  deleteShellSession,
  getAgents,
  listShellSessions,
} from "../components/Api";

function shortUuid(uuid: string): string {
  return uuid.length > 12 ? `${uuid.slice(0, 8)}...${uuid.slice(-4)}` : uuid;
}

export default function Home() {
  const queryClient = useQueryClient();

  const [isAddAgentDialogOpen, setIsAddAgentDialogOpen] = useState(false);
  const [name, setName] = useState("");
  const [region, setRegion] = useState("us-east-1");
  const [instanceType, setInstanceType] = useState("t4g.nano");
  const [architecture, setArchitecture] = useState<"x86" | "arm">("arm");

  const [agentPendingDelete, setAgentPendingDelete] = useState<string | null>(
    null,
  );
  const [creatingShellForAgent, setCreatingShellForAgent] = useState<
    string | null
  >(null);
  const [deletingShellUuid, setDeletingShellUuid] = useState<string | null>(
    null,
  );
  const [statusMessage, setStatusMessage] = useState<string | null>(null);

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

  const shellsByAgent = useMemo(() => {
    const grouped = new Map<
      string,
      Array<{ uuid: string; agent_name: string }>
    >();
    for (const session of shellSessions ?? []) {
      const list = grouped.get(session.agent_name) ?? [];
      list.push(session);
      grouped.set(session.agent_name, list);
    }
    return grouped;
  }, [shellSessions]);

  const createAgentMutation = useMutation({
    mutationFn: (payload: {
      name: string;
      region: string;
      instanceType: string;
      architecture: "x86" | "arm";
    }) => createAgent(payload.name, payload.region, payload.instanceType, payload.architecture),
    onSuccess: (_, payload) => {
      queryClient.invalidateQueries({ queryKey: ["agents"] });
      setStatusMessage(`Agent creation request sent: ${payload.name}.`);
      setIsAddAgentDialogOpen(false);
      setName("");
      setRegion("us-east-1");
      setInstanceType("t4g.nano");
      setArchitecture("arm");
    },
    onError: (error) => {
      setStatusMessage(`Failed to create agent: ${(error as Error).message}`);
    },
  });

  const deleteAgentMutation = useMutation({
    mutationFn: (agentName: string) => deleteAgent(agentName),
    onSuccess: (_, agentName) => {
      queryClient.invalidateQueries({ queryKey: ["agents"] });
      queryClient.invalidateQueries({ queryKey: ["shellSessions"] });
      setStatusMessage(`Deleted agent ${agentName}.`);
      setAgentPendingDelete(null);
    },
    onError: (error) => {
      setStatusMessage(`Failed to delete agent: ${(error as Error).message}`);
    },
  });

  const createShellSessionMutation = useMutation({
    mutationFn: (agentName: string) => createShellSession(agentName),
    onSuccess: (uuid, agentName) => {
      queryClient.invalidateQueries({ queryKey: ["shellSessions"] });
      setStatusMessage(`Created shell ${uuid} for ${agentName}.`);
    },
    onError: (error) => {
      setStatusMessage(`Failed to create shell: ${(error as Error).message}`);
    },
    onSettled: () => {
      setCreatingShellForAgent(null);
    },
  });

  const deleteShellSessionMutation = useMutation({
    mutationFn: (uuid: string) => deleteShellSession(uuid),
    onSuccess: (_, uuid) => {
      queryClient.invalidateQueries({ queryKey: ["shellSessions"] });
      setStatusMessage(`Deleted shell ${uuid}.`);
    },
    onError: (error) => {
      setStatusMessage(`Failed to delete shell: ${(error as Error).message}`);
    },
    onSettled: () => {
      setDeletingShellUuid(null);
    },
  });

  function handleCreateAgent(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setStatusMessage(null);
    createAgentMutation.mutate({ name, region, instanceType, architecture });
  }

  function handleConfirmDeleteAgent() {
    if (!agentPendingDelete) {
      return;
    }
    setStatusMessage(null);
    deleteAgentMutation.mutate(agentPendingDelete);
  }

  function handleCreateShell(agentName: string) {
    if (
      createShellSessionMutation.isPending ||
      deleteShellSessionMutation.isPending
    ) {
      return;
    }
    setStatusMessage(null);
    setCreatingShellForAgent(agentName);
    createShellSessionMutation.mutate(agentName);
  }

  function handleDeleteShell(uuid: string) {
    if (
      deleteShellSessionMutation.isPending ||
      createShellSessionMutation.isPending
    ) {
      return;
    }
    setStatusMessage(null);
    setDeletingShellUuid(uuid);
    deleteShellSessionMutation.mutate(uuid);
  }

  useEffect(() => {
    if (!isAddAgentDialogOpen) {
      return;
    }

    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !createAgentMutation.isPending) {
        setIsAddAgentDialogOpen(false);
      }
    };

    window.addEventListener("keydown", handleEscape);
    return () => window.removeEventListener("keydown", handleEscape);
  }, [isAddAgentDialogOpen, createAgentMutation.isPending]);

  useEffect(() => {
    if (!agentPendingDelete) {
      return;
    }

    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !deleteAgentMutation.isPending) {
        setAgentPendingDelete(null);
      }
    };

    window.addEventListener("keydown", handleEscape);
    return () => window.removeEventListener("keydown", handleEscape);
  }, [agentPendingDelete, deleteAgentMutation.isPending]);

  if (isAgentsLoading || isShellSessionsLoading) {
    return <div className="container mt-4">Loading hub...</div>;
  }

  if (agentsError || shellSessionsError) {
    return (
      <div className="container mt-4">
        <div className="alert alert-danger mb-0">
          {agentsError ? (
            <>Failed to fetch agents: {(agentsError as Error).message}</>
          ) : (
            <>
              Failed to fetch shell sessions:{" "}
              {(shellSessionsError as Error).message}
            </>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="container mt-4">
      <div className="d-flex justify-content-between align-items-center mb-3">
        <button
          type="button"
          className="btn btn-primary"
          onClick={() => setIsAddAgentDialogOpen(true)}
        >
          Add Agent
        </button>
      </div>

      {statusMessage ? (
        <div className="alert alert-info dark-text py-2">{statusMessage}</div>
      ) : null}

      {!agents || agents.length === 0 ? (
        <p>No agents found. Add one to get started.</p>
      ) : (
        <div className="row g-3">
          {agents.map((agent) => {
            const agentShells = shellsByAgent.get(agent.name) ?? [];
            return (
              <div key={agent.name} className="col-12">
                <div className="card">
                  <div className="card-body">
                    <div className="d-flex justify-content-between align-items-start gap-3">
                      <div>
                        <h5 className="card-title mb-1">{agent.name}</h5>
                        <div className="small text-muted">
                          {agent.config.instance_type} in {agent.config.region}
                        </div>
                        <div className="small text-muted">
                          AMI: {agent.config.ami_id}
                        </div>
                        <div className="small text-muted">
                          {agent.public_ip_address
                            ? `Public IP: ${agent.public_ip_address}`
                            : "No public IP address"}
                        </div>
                      </div>
                      <div className="d-flex align-items-center gap-2">
                        <span className="badge text-bg-secondary">
                          {agent.state}
                        </span>
                        <button
                          type="button"
                          className="btn btn-outline-danger btn-sm"
                          title={`Delete ${agent.name}`}
                          onClick={() => setAgentPendingDelete(agent.name)}
                          disabled={deleteAgentMutation.isPending}
                        >
                          X
                        </button>
                      </div>
                    </div>

                    <div className="d-flex align-items-start gap-2 mt-3">
                      <details className="flex-grow-1">
                        <summary className="btn btn-outline-secondary btn-sm">
                          Shells ({agentShells.length})
                        </summary>
                        <div className="list-group mt-2">
                          {agentShells.length === 0 ? (
                            <div className="list-group-item text-muted small">
                              No shells for this agent.
                            </div>
                          ) : (
                            agentShells.map((shell) => (
                              <div
                                key={shell.uuid}
                                className="list-group-item d-flex justify-content-between align-items-center"
                              >
                                <Link to={`/shell/${shell.uuid}`}>
                                  {shortUuid(shell.uuid)}
                                </Link>
                                <button
                                  type="button"
                                  className="btn btn-link p-0 text-danger"
                                  title="Delete shell session"
                                  onClick={() => handleDeleteShell(shell.uuid)}
                                  disabled={
                                    deleteShellSessionMutation.isPending ||
                                    createShellSessionMutation.isPending
                                  }
                                >
                                  {deleteShellSessionMutation.isPending &&
                                  deletingShellUuid === shell.uuid
                                    ? "Deleting..."
                                    : "X"}
                                </button>
                              </div>
                            ))
                          )}
                        </div>
                      </details>

                      <button
                        type="button"
                        className="btn btn-outline-success btn-sm"
                        onClick={() => handleCreateShell(agent.name)}
                        disabled={
                          createShellSessionMutation.isPending ||
                          deleteShellSessionMutation.isPending
                        }
                      >
                        {createShellSessionMutation.isPending &&
                        creatingShellForAgent === agent.name
                          ? "Adding shell..."
                          : "+"}
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {isAddAgentDialogOpen ? (
        <div
          className="shell-transfer-dialog-backdrop"
          role="presentation"
          onClick={() => {
            if (!createAgentMutation.isPending) {
              setIsAddAgentDialogOpen(false);
            }
          }}
        >
          <div
            className="shell-transfer-dialog card"
            role="dialog"
            aria-modal="true"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="card-body">
              <h5 className="card-title mb-3">Add Agent</h5>
              <form onSubmit={handleCreateAgent}>
                <div className="mb-2">
                  <label
                    className="form-label small mb-1"
                    htmlFor="new-agent-name"
                  >
                    Agent Name
                  </label>
                  <input
                    id="new-agent-name"
                    className="form-control form-control-sm"
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    placeholder="agent-01"
                    required
                  />
                </div>
                <div className="mb-2">
                  <label
                    className="form-label small mb-1"
                    htmlFor="new-agent-region"
                  >
                    Region
                  </label>
                  <input
                    id="new-agent-region"
                    className="form-control form-control-sm"
                    value={region}
                    onChange={(event) => setRegion(event.target.value)}
                    placeholder="us-east-1"
                    required
                  />
                </div>
                <div className="mb-2">
                  <label
                    className="form-label small mb-1"
                    htmlFor="new-agent-instance"
                  >
                    Instance Type
                  </label>
                  <input
                    id="new-agent-instance"
                    className="form-control form-control-sm"
                    value={instanceType}
                    onChange={(event) => setInstanceType(event.target.value)}
                    placeholder="t4g.nano"
                    required
                  />
                </div>
                <div className="mb-3">
                  <label
                    className="form-label small mb-1"
                    htmlFor="new-agent-architecture"
                  >
                    Architecture
                  </label>
                  <select
                    id="new-agent-architecture"
                    className="form-select form-select-sm"
                    value={architecture}
                    onChange={(event) => setArchitecture(event.target.value as "x86" | "arm")}
                  >
                    <option value="arm">arm (ARM64)</option>
                    <option value="x86">x86 (x86_64)</option>
                  </select>
                </div>
                <div className="d-flex justify-content-end gap-2">
                  <button
                    type="button"
                    className="btn btn-secondary btn-sm"
                    onClick={() => setIsAddAgentDialogOpen(false)}
                    disabled={createAgentMutation.isPending}
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    className="btn btn-primary btn-sm"
                    disabled={createAgentMutation.isPending}
                  >
                    {createAgentMutation.isPending ? "Creating..." : "Create"}
                  </button>
                </div>
              </form>
            </div>
          </div>
        </div>
      ) : null}

      {agentPendingDelete ? (
        <div
          className="shell-transfer-dialog-backdrop"
          role="presentation"
          onClick={() => {
            if (!deleteAgentMutation.isPending) {
              setAgentPendingDelete(null);
            }
          }}
        >
          <div
            className="shell-transfer-dialog card"
            role="dialog"
            aria-modal="true"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="card-body">
              <h5 className="card-title mb-2">Delete Agent</h5>
              <p className="text-muted small mb-3">
                Are you sure you want to delete{" "}
                <strong>{agentPendingDelete}</strong>?
              </p>
              <div className="d-flex justify-content-end gap-2">
                <button
                  type="button"
                  className="btn btn-secondary btn-sm"
                  onClick={() => setAgentPendingDelete(null)}
                  disabled={deleteAgentMutation.isPending}
                >
                  Cancel
                </button>
                <button
                  type="button"
                  className="btn btn-danger btn-sm"
                  onClick={handleConfirmDeleteAgent}
                  disabled={deleteAgentMutation.isPending}
                >
                  {deleteAgentMutation.isPending ? "Deleting..." : "Delete"}
                </button>
              </div>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
