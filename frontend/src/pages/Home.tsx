import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { getAgents, listShellSessions } from "../components/Api";
import AddAgentDialog from "../components/AddAgentDialog";
import AgentCard from "../components/AgentCard";
import DeleteAgentDialog from "../components/DeleteAgentDialog";
import RunCommandDialog from "../components/RunCommandDialog";

export default function Home() {
  const [isAddAgentDialogOpen, setIsAddAgentDialogOpen] = useState(false);
  const [agentPendingDelete, setAgentPendingDelete] = useState<string | null>(null);
  const [runCommandAgent, setRunCommandAgent] = useState<string | null>(null);
  const [statusMessage, setStatusMessage] = useState<{
    text: string;
    isError: boolean;
  } | null>(null);

  function handleStatusMessage(msg: string, isError = false) {
    setStatusMessage({ text: msg, isError });
  }

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
    const grouped = new Map<string, Array<{ uuid: string; agent_name: string }>>();
    for (const session of shellSessions ?? []) {
      const list = grouped.get(session.agent_name) ?? [];
      list.push(session);
      grouped.set(session.agent_name, list);
    }
    return grouped;
  }, [shellSessions]);

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
        <div
          className={`alert py-2 d-flex justify-content-between align-items-center ${statusMessage.isError ? "alert-danger" : "alert-info dark-text"}`}
        >
          <span>{statusMessage.text}</span>
          <button
            type="button"
            className="btn-close"
            aria-label="Close"
            onClick={() => setStatusMessage(null)}
          />
        </div>
      ) : null}

      {!agents || agents.length === 0 ? (
        <p>No agents found. Add one to get started.</p>
      ) : (
        <div className="row g-3">
          {agents.map((agent) => (
            <AgentCard
              key={agent.name}
              agent={agent}
              shells={shellsByAgent.get(agent.name) ?? []}
              onDeleteClick={setAgentPendingDelete}
              onRunCommandClick={setRunCommandAgent}
              onStatusMessage={handleStatusMessage}
            />
          ))}
        </div>
      )}

      {isAddAgentDialogOpen ? (
        <AddAgentDialog
          onClose={() => setIsAddAgentDialogOpen(false)}
          onStatusMessage={handleStatusMessage}
        />
      ) : null}

      {agentPendingDelete ? (
        <DeleteAgentDialog
          agentName={agentPendingDelete}
          onClose={() => setAgentPendingDelete(null)}
          onStatusMessage={handleStatusMessage}
        />
      ) : null}

      {runCommandAgent ? (
        <RunCommandDialog
          agentName={runCommandAgent}
          onClose={() => setRunCommandAgent(null)}
          onStatusMessage={handleStatusMessage}
        />
      ) : null}
    </div>
  );
}
