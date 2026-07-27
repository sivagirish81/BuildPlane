import type {
  AuditRecord,
  ComponentDependency,
  ComponentVersion,
  EvaluationRun,
  HumanDecisionResponse,
  PromotionResponse,
  WorkflowRun,
} from "./types";

const API_BASE_URL = import.meta.env.VITE_BUILDPLANE_API_BASE_URL ?? "/api";

type RequestOptions = RequestInit & {
  idempotencyKey?: string;
};

async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers(options.headers);
  if (options.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  if (options.idempotencyKey) {
    headers.set("Idempotency-Key", options.idempotencyKey);
  }

  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers,
  });
  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`;
    try {
      const body = (await response.json()) as { error?: { message?: string } };
      message = body.error?.message ?? message;
    } catch {
      // Keep the HTTP status message when the server did not return JSON.
    }
    throw new Error(message);
  }
  return response.json() as Promise<T>;
}

export function eventSourceURL(path: string): string {
  return `${API_BASE_URL}${path}`;
}

export async function getHealth(): Promise<{ status: string }> {
  return apiRequest<{ status: string }>("/healthz");
}

export async function listWorkflowRuns(): Promise<WorkflowRun[]> {
  const body = await apiRequest<{ workflow_runs: WorkflowRun[] }>("/v1/workflow-runs?limit=50");
  return body.workflow_runs;
}

export async function getWorkflowRun(id: string): Promise<WorkflowRun> {
  const body = await apiRequest<{ workflow_run: WorkflowRun }>(`/v1/workflow-runs/${id}`);
  return body.workflow_run;
}

export async function getWorkflowAudit(id: string): Promise<AuditRecord[]> {
  const body = await apiRequest<{ audit_records: AuditRecord[] }>(`/v1/workflow-runs/${id}/audit`);
  return body.audit_records;
}

export async function createWorkflowRun(workflowName: string, input: Record<string, unknown>): Promise<WorkflowRun> {
  const body = await apiRequest<{ workflow_run: WorkflowRun }>("/v1/workflow-runs", {
    method: "POST",
    idempotencyKey: `web-${workflowName}-${Date.now()}`,
    body: JSON.stringify({
      workflow_name: workflowName,
      input,
    }),
  });
  return body.workflow_run;
}

export async function submitHumanDecision(
  workflowRunId: string,
  decision: "approved" | "rejected",
  actorId: string,
  reason: string,
): Promise<HumanDecisionResponse> {
  return apiRequest<HumanDecisionResponse>(`/v1/workflow-runs/${workflowRunId}/decisions`, {
    method: "POST",
    idempotencyKey: `web-decision-${workflowRunId}-${decision}-${Date.now()}`,
    body: JSON.stringify({
      decision,
      actor_id: actorId,
      reason,
    }),
  });
}

export async function listComponentVersions(componentName = "issue_classifier"): Promise<ComponentVersion[]> {
  const body = await apiRequest<{ component_versions: ComponentVersion[] }>(
    `/v1/component-versions?component_name=${encodeURIComponent(componentName)}&limit=25`,
  );
  return body.component_versions;
}

export async function listAffectedWorkflows(componentName = "issue_classifier"): Promise<ComponentDependency[]> {
  const body = await apiRequest<{ affected_workflows: ComponentDependency[] }>(
    `/v1/components/${encodeURIComponent(componentName)}/affected-workflows`,
  );
  return body.affected_workflows;
}

export async function createComponentVersion(
  version: string,
  promptVersion: string,
  changeSummary: string,
  spec: Record<string, unknown>,
): Promise<ComponentVersion> {
  const body = await apiRequest<{ component_version: ComponentVersion }>("/v1/component-versions", {
    method: "POST",
    body: JSON.stringify({
      component_name: "issue_classifier",
      version,
      prompt_version: promptVersion,
      spec,
      change_summary: changeSummary,
      created_by: "web-operator",
    }),
  });
  return body.component_version;
}

export async function runEvaluation(id: string): Promise<EvaluationRun> {
  const body = await apiRequest<{ evaluation_run: EvaluationRun }>(`/v1/component-versions/${id}/evaluations`, {
    method: "POST",
  });
  return body.evaluation_run;
}

export async function startCanary(id: string, percent: number): Promise<ComponentVersion> {
  const body = await apiRequest<{ component_version: ComponentVersion }>(`/v1/component-versions/${id}/canary`, {
    method: "POST",
    body: JSON.stringify({ percent }),
  });
  return body.component_version;
}

export async function promoteVersion(id: string): Promise<PromotionResponse> {
  return apiRequest<PromotionResponse>(`/v1/component-versions/${id}/promote`, {
    method: "POST",
  });
}

export async function rollbackIssueClassifier(reason: string): Promise<PromotionResponse> {
  return apiRequest<PromotionResponse>("/v1/components/issue_classifier/rollback", {
    method: "POST",
    body: JSON.stringify({
      actor_id: "web-operator",
      reason,
    }),
  });
}
