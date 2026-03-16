import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { deleteAgent } from "./Api";

interface DeleteAgentDialogProps {
  agentName: string;
  onClose: () => void;
  onStatusMessage: (msg: string, isError?: boolean) => void;
}

export default function DeleteAgentDialog({
  agentName,
  onClose,
  onStatusMessage,
}: DeleteAgentDialogProps) {
  const queryClient = useQueryClient();

  const deleteAgentMutation = useMutation({
    mutationFn: () => deleteAgent(agentName),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["agents"] });
      queryClient.invalidateQueries({ queryKey: ["shellSessions"] });
      onStatusMessage(`Deleted agent ${agentName}.`);
      onClose();
    },
    onError: (error) => {
      onStatusMessage(`Failed to delete agent: ${(error as Error).message}`, true);
    },
  });

  useEffect(() => {
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !deleteAgentMutation.isPending) {
        onClose();
      }
    };
    window.addEventListener("keydown", handleEscape);
    return () => window.removeEventListener("keydown", handleEscape);
  }, [deleteAgentMutation.isPending, onClose]);

  return (
    <div
      className="shell-transfer-dialog-backdrop"
      role="presentation"
      onClick={() => {
        if (!deleteAgentMutation.isPending) {
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
          <h5 className="card-title mb-2">Delete Agent</h5>
          <p className="text-muted small mb-3">
            Are you sure you want to delete <strong>{agentName}</strong>?
          </p>
          <div className="d-flex justify-content-end gap-2">
            <button
              type="button"
              className="btn btn-secondary btn-sm"
              onClick={onClose}
              disabled={deleteAgentMutation.isPending}
            >
              Cancel
            </button>
            <button
              type="button"
              className="btn btn-danger btn-sm"
              onClick={() => deleteAgentMutation.mutate()}
              disabled={deleteAgentMutation.isPending}
            >
              {deleteAgentMutation.isPending ? "Deleting..." : "Delete"}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
