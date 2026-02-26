import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { deleteAgent, getAgents } from "../components/Api";

export default function DeleteAgent() {
  const queryClient = useQueryClient();

  const {
    data: agents,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["agents"],
    queryFn: () => getAgents(),
    refetchInterval: 5000,
  });

  const deleteAgentMutation = useMutation({
    mutationFn: (name: string) => deleteAgent(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["agents"] });
    },
  });

  function onDelete(name: string) {
    deleteAgentMutation.mutate(name);
  }

  return (
    <div className="container mt-4">
      <h1>Delete Agent</h1>
      <p className="text-muted">Select an agent and remove it.</p>

      {isLoading ? <p>Loading agents...</p> : null}

      {error ? (
        <div className="alert alert-danger">
          Failed to fetch agents: {(error as Error).message}
        </div>
      ) : null}

      {!isLoading && !error && agents?.length === 0 ? (
        <p>No agents available to delete.</p>
      ) : null}

      {!isLoading && !error && agents && agents.length > 0 ? (
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
                className="btn btn-danger btn-sm"
                onClick={() => onDelete(agent.name)}
                disabled={deleteAgentMutation.isPending}
              >
                {deleteAgentMutation.isPending ? "Deleting..." : "Delete"}
              </button>
            </div>
          ))}
        </div>
      ) : null}

      {deleteAgentMutation.error ? (
        <div className="alert alert-danger mt-3 mb-0">
          Failed to delete agent: {(deleteAgentMutation.error as Error).message}
        </div>
      ) : null}
    </div>
  );
}
