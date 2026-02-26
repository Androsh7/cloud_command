import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { createAgent } from "../components/Api";

export default function CreateAgent() {
  const queryClient = useQueryClient();
  const [name, setName] = useState("");
  const [region, setRegion] = useState("us-east-1");
  const [instanceType, setInstanceType] = useState("t4g.nano");
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  const createAgentMutation = useMutation({
    mutationFn: ({
      name,
      region,
      instanceType,
    }: {
      name: string;
      region: string;
      instanceType: string;
    }) => createAgent(name, region, instanceType),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ["agents"] });
      setSuccessMessage(`Agent creation request sent: ${variables.name}.`);
    },
    onError: () => {
      setSuccessMessage(null);
    },
  });

  function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    createAgentMutation.mutate({ name, region, instanceType });
  }

  return (
    <div className="container mt-4">
      <h1>Create Agent</h1>
      <p className="text-muted">Create a new EC2-backed agent.</p>

      <form className="card p-3" onSubmit={handleSubmit}>
        <div className="mb-3">
          <label className="form-label" htmlFor="name">
            Agent Name
          </label>
          <input
            id="name"
            className="form-control"
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="agent-01"
            required
          />
        </div>

        <div className="mb-3">
          <label className="form-label" htmlFor="region">
            AWS Region
          </label>
          <input
            id="region"
            className="form-control"
            value={region}
            onChange={(event) => setRegion(event.target.value)}
            placeholder="us-east-1"
            required
          />
        </div>

        <div className="mb-3">
          <label className="form-label" htmlFor="instanceType">
            Instance Type
          </label>
          <input
            id="instanceType"
            className="form-control"
            value={instanceType}
            onChange={(event) => setInstanceType(event.target.value)}
            placeholder="t2.micro"
            required
          />
        </div>

        <button
          type="submit"
          className="btn btn-primary"
          disabled={createAgentMutation.isPending}
        >
          {createAgentMutation.isPending ? "Creating..." : "Create Agent"}
        </button>
      </form>

      {successMessage ? (
        <div className="alert alert-success mt-3 mb-0">{successMessage}</div>
      ) : null}

      {createAgentMutation.error ? (
        <div className="alert alert-danger mt-3 mb-0">
          Failed to create agent: {(createAgentMutation.error as Error).message}
        </div>
      ) : null}
    </div>
  );
}
