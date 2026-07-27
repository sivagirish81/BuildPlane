import type { ComponentStatus, WorkflowStatus } from "./types";

export function statusLabel(status: WorkflowStatus | ComponentStatus): string {
  return status.replace(/_/g, " ");
}

export function statusTone(status: WorkflowStatus | ComponentStatus): string {
  switch (status) {
    case "succeeded":
    case "promoted":
      return "tone-good";
    case "waiting_for_human":
    case "canary":
    case "evaluated":
      return "tone-warn";
    case "failed":
    case "canceled":
    case "rolled_back":
      return "tone-bad";
    default:
      return "tone-neutral";
  }
}

export function formatJSON(value: unknown): string {
  return JSON.stringify(value ?? {}, null, 2);
}

export function compactID(id: string): string {
  if (id.length <= 13) {
    return id;
  }
  return `${id.slice(0, 8)}...${id.slice(-4)}`;
}
