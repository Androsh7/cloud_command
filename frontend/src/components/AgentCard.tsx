import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link } from "react-router-dom";
import { createShellSession, deleteShellSession } from "./Api";
import type { AgentModel, ShellSessionModel } from "./Models";

function shortUuid(uuid: string): string {
  return uuid.length > 12 ? `${uuid.slice(0, 8)}...${uuid.slice(-4)}` : uuid;
}

interface AgentCardProps {
  agent: AgentModel;
  shells: ShellSessionModel[];
  onDeleteClick: (agentName: string) => void;
  onRunCommandClick: (agentName: string) => void;
  onStatusMessage: (msg: string, isError?: boolean) => void;
}

export default function AgentCard({
  agent,
  shells,
  onDeleteClick,
  onRunCommandClick,
  onStatusMessage,
}: AgentCardProps) {
  const queryClient = useQueryClient();

  const [creatingShell, setCreatingShell] = useState(false);
  const [deletingShellUuid, setDeletingShellUuid] = useState<string | null>(null);

  const createShellSessionMutation = useMutation({
    mutationFn: () => createShellSession(agent.name),
    onSuccess: (uuid) => {
      queryClient.invalidateQueries({ queryKey: ["shellSessions"] });
      onStatusMessage(`Created shell ${uuid} for ${agent.name}.`);
    },
    onError: (error) => {
      onStatusMessage(`Failed to create shell: ${(error as Error).message}`, true);
    },
    onSettled: () => {
      setCreatingShell(false);
    },
  });

  const deleteShellSessionMutation = useMutation({
    mutationFn: (uuid: string) => deleteShellSession(uuid),
    onSuccess: (_, uuid) => {
      queryClient.invalidateQueries({ queryKey: ["shellSessions"] });
      onStatusMessage(`Deleted shell ${uuid}.`);
    },
    onError: (error) => {
      onStatusMessage(`Failed to delete shell: ${(error as Error).message}`, true);
    },
    onSettled: () => {
      setDeletingShellUuid(null);
    },
  });

  function handleCreateShell() {
    if (createShellSessionMutation.isPending || deleteShellSessionMutation.isPending) {
      return;
    }
    setCreatingShell(true);
    createShellSessionMutation.mutate();
  }

  function handleDeleteShell(uuid: string) {
    if (deleteShellSessionMutation.isPending || createShellSessionMutation.isPending) {
      return;
    }
    setDeletingShellUuid(uuid);
    deleteShellSessionMutation.mutate(uuid);
  }

  return (
    <div className="col-12">
      <div className="card">
        <div className="card-body">
          <div className="d-flex justify-content-between align-items-start gap-3">
            <div>
              <h5 className="card-title mb-1">{agent.name}</h5>
              <div className="small text-muted">
                {agent.config.instance_type} in {agent.config.region}
              </div>
              <div className="small text-muted">AMI: {agent.config.ami_id}</div>
              <div className="small text-muted">
                {agent.public_ip_address
                  ? `Public IP: ${agent.public_ip_address}`
                  : "No public IP address"}
              </div>
            </div>
            <div className="d-flex align-items-center gap-2">
              <span className="badge text-bg-secondary">{agent.state}</span>
              <button
                type="button"
                className="btn btn-outline-danger btn-sm"
                title={`Delete ${agent.name}`}
                onClick={() => onDeleteClick(agent.name)}
              >
                X
              </button>
            </div>
          </div>

          <div className="d-flex align-items-start gap-2 mt-3">
            <details className="flex-grow-1">
              <summary className="btn btn-outline-secondary btn-sm">
                Shells ({shells.length})
              </summary>
              <div className="list-group mt-2">
                {shells.length === 0 ? (
                  <div className="list-group-item text-muted small">
                    No shells for this agent.
                  </div>
                ) : (
                  shells.map((shell) => (
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
              onClick={handleCreateShell}
              disabled={
                createShellSessionMutation.isPending ||
                deleteShellSessionMutation.isPending
              }
            >
              {creatingShell && createShellSessionMutation.isPending
                ? "Adding shell..."
                : "+"}
            </button>
            <button
              type="button"
              className="btn btn-outline-secondary btn-sm"
              title={`Run command on ${agent.name}`}
              onClick={() => onRunCommandClick(agent.name)}
            >
              &gt;_
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
