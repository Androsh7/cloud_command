import { useQuery } from "@tanstack/react-query";
import { getAgents } from "../components/Api";

export default function Home() {
  const {
    data: agents,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["agents"],
    queryFn: () => getAgents(),
    refetchInterval: 5000,
  });
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
              className="list-group-item list-group-item-action flex-column align-items-start"
            >
              <div className="d-flex w-100 justify-content-between">
                <h5 className="mb-1">{agent.name}</h5>
                <small>{agent.state}</small>
              </div>
              <p className="mb-1">
                {agent.public_ip_address
                  ? `Public IP: ${agent.public_ip_address}`
                  : "No public IP address"}
              </p>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
