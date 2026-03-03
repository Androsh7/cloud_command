import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { createShellSession, getAgents } from "../components/Api";

export default function Home() {
  const navigate = useNavigate();
  const [launchingAgentName, setLaunchingAgentName] = useState<string | null>(
    null,
  );
  const {
    data: agents,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["agents"],
    queryFn: () => getAgents(),
    refetchInterval: 5000,
  });

  const createShellSessionMutation = useMutation({
    mutationFn: (agentName: string) => createShellSession(agentName),
    onSuccess: (uuid) => {
      navigate(`/shell/${uuid}`);
    },
    onSettled: () => {
      setLaunchingAgentName(null);
    },
  });

  function handleOpenShell(agentName: string) {
    if (createShellSessionMutation.isPending) {
      return;
    }
    setLaunchingAgentName(agentName);
    createShellSessionMutation.mutate(agentName);
  }

  if (isLoading) {
    return <div className="container mt-4">Loading...</div>;
  }
  if (error) {
    return (
      <div className="container mt-4">
        Error fetching agents: {(error as Error).message}
      </div>
    );
  }
  return (
    <div className="container mt-4">
      <h1>Agents</h1>
      {agents?.length === 0 ? (
        <p>No agents found. Create one to get started.</p>
      ) : (
        <div className="list-group">
          {agents?.map((agent) => (
            <div
              key={agent.name}
              className="list-group-item flex-column align-items-start"
            >
              <div className="d-flex w-100 justify-content-between">
                <h5 className="mb-1">
                  {agent.name} ({agent.config.ami_id})
                </h5>
                <small>
                  ({agent.config.instance_type} in {agent.config.region})
                </small>
                <small>{agent.state}</small>
              </div>
              <p className="mb-1">
                {agent.public_ip_address
                  ? `Public IP: ${agent.public_ip_address}`
                  : "No public IP address"}
              </p>
              <button
                type="button"
                className="btn btn-primary"
                onClick={() => handleOpenShell(agent.name)}
                disabled={createShellSessionMutation.isPending}
              >
                {createShellSessionMutation.isPending &&
                launchingAgentName === agent.name
                  ? "Opening..."
                  : "Open Shell"}
              </button>
            </div>
          ))}
        </div>
      )}
      {createShellSessionMutation.error ? (
        <div className="alert alert-danger mt-3 mb-0">
          Failed to open shell:{" "}
          {(createShellSessionMutation.error as Error).message}
        </div>
      ) : null}
    </div>
  );
}
