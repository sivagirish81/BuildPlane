export type WorkflowStatus =
  | "queued"
  | "running"
  | "waiting_for_human"
  | "succeeded"
  | "failed"
  | "canceled";

export type WorkflowRun = {
  id: string;
  workflow_name: string;
  status: WorkflowStatus;
  input: Record<string, unknown>;
  correlation_id: string;
  traceparent: string;
  created_at: string;
  updated_at: string;
};

export type AuditRecord = {
  id: number;
  workflow_run_id: string;
  node_execution_id?: string;
  event_type: string;
  actor_type: string;
  actor_id: string;
  details: Record<string, unknown>;
  created_at: string;
};

export type WorkflowRunEvent = {
  workflow_run: WorkflowRun;
  audit_records: AuditRecord[];
  sent_at: string;
};

export type HumanDecisionResponse = {
  workflow_run: WorkflowRun;
  next_node_name?: string;
  replayed: boolean;
};

export type ComponentStatus =
  | "candidate"
  | "evaluated"
  | "canary"
  | "promoted"
  | "superseded"
  | "rolled_back";

export type ComponentVersion = {
  id: string;
  component_name: string;
  version: string;
  prompt_version: string;
  spec: Record<string, unknown>;
  status: ComponentStatus;
  change_summary: string;
  created_by: string;
  canary_percent: number;
  evaluation_passed: boolean;
  previous_promoted_version_id?: string;
  created_at: string;
  updated_at: string;
};

export type EvaluationRun = {
  id: string;
  component_version_id: string;
  component_name: string;
  baseline_version_id: string;
  dataset_name: string;
  status: string;
  candidate_passed: boolean;
  baseline_passed: boolean;
  candidate_passed_cases: number;
  candidate_total_cases: number;
  baseline_passed_cases: number;
  baseline_total_cases: number;
  summary: Record<string, unknown>;
  created_at: string;
};

export type PromotionResponse = {
  promotion: {
    promoted: ComponentVersion;
    previous: ComponentVersion;
  };
};

export type ComponentDependency = {
  workflow_name: string;
  node_name: string;
  component_name: string;
};
