import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState, type FormEvent } from "react";
import { createAgent } from "./Api";

interface AddAgentDialogProps {
  onClose: () => void;
  onStatusMessage: (msg: string) => void;
}

export default function AddAgentDialog({
  onClose,
  onStatusMessage,
}: AddAgentDialogProps) {
  const queryClient = useQueryClient();

  const [name, setName] = useState("");
  const [region, setRegion] = useState("us-east-1");
  const [instanceType, setInstanceType] = useState("t4g.nano");
  const [architecture, setArchitecture] = useState<"x86_64" | "arm64">("arm64");

  const createAgentMutation = useMutation({
    mutationFn: (payload: {
      name: string;
      region: string;
      instanceType: string;
      architecture: "x86_64" | "arm64";
    }) =>
      createAgent(
        payload.name,
        payload.region,
        payload.instanceType,
        payload.architecture,
      ),
    onSuccess: (_, payload) => {
      queryClient.invalidateQueries({ queryKey: ["agents"] });
      onStatusMessage(`Agent creation request sent: ${payload.name}.`);
      onClose();
    },
    onError: (error) => {
      onStatusMessage(`Failed to create agent: ${(error as Error).message}`);
    },
  });

  function handleCreateAgent(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    createAgentMutation.mutate({ name, region, instanceType, architecture });
  }

  useEffect(() => {
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !createAgentMutation.isPending) {
        onClose();
      }
    };
    window.addEventListener("keydown", handleEscape);
    return () => window.removeEventListener("keydown", handleEscape);
  }, [createAgentMutation.isPending, onClose]);

  return (
    <div
      className="shell-transfer-dialog-backdrop"
      role="presentation"
      onClick={() => {
        if (!createAgentMutation.isPending) {
          onClose();
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
                onChange={(event) =>
                  setArchitecture(event.target.value as "x86_64" | "arm64")
                }
              >
                <option value="arm64">arm64</option>
                <option value="x86_64">x86_64</option>
              </select>
            </div>
            <div className="d-flex justify-content-end gap-2">
              <button
                type="button"
                className="btn btn-secondary btn-sm"
                onClick={onClose}
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
  );
}
