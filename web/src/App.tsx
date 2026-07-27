import { useEffect, useMemo, useState } from "react";
import {
  Activity,
  CheckCircle2,
  FlaskConical,
  GitBranch,
  Play,
  RefreshCw,
  Rocket,
  RotateCcw,
  ShieldCheck,
  XCircle,
} from "lucide-react";
import {
  createComponentVersion,
  createWorkflowRun,
  eventSourceURL,
  getHealth,
  getWorkflowAudit,
  listAffectedWorkflows,
  listComponentVersions,
  listWorkflowRuns,
  promoteVersion,
  rollbackIssueClassifier,
  runEvaluation,
  startCanary,
  submitHumanDecision,
} from "./api";
import type { AuditRecord, ComponentDependency, ComponentVersion, WorkflowRun, WorkflowRunEvent } from "./types";
import { compactID, formatJSON, statusLabel, statusTone } from "./viewModel";

type Tab = "workflows" | "components";

const invoiceDemoInput = {
  case_id: "synthetic-inv-case-001",
  title: "Invoice price mismatch",
  description: "Synthetic invoice total is higher than purchase order",
  customer_message: "Please review this invoice before payment",
  source: "web-console",
  invoice_id: "synthetic-inv-001",
  vendor_name: "Synthetic Vendor",
  amount_disputed: 1250.5,
};

const freightDemoInput = {
  case_id: "synthetic-fr-case-001",
  title: "Freight delivery exception",
  description: "Synthetic carrier missed the delivery window",
  customer_message: "Please review the freight exception before escalation",
  source: "web-console",
  shipment_id: "synthetic-fr-001",
  carrier_name: "Synthetic Carrier",
  days_late: 3,
};

const candidateSpec = {
  rules: [
    { category: "billing", keywords: ["invoice", "payment", "charge", "credit"] },
    { category: "logistics", keywords: ["freight", "shipment", "carrier", "delivery", "late"] },
    { category: "access", keywords: ["access", "login", "portal", "permission"] },
  ],
  default_category: "general",
};

export function App() {
  const [tab, setTab] = useState<Tab>("workflows");
  const [health, setHealth] = useState("checking");
  const [runs, setRuns] = useState<WorkflowRun[]>([]);
  const [selectedRunID, setSelectedRunID] = useState("");
  const [selectedRun, setSelectedRun] = useState<WorkflowRun | null>(null);
  const [auditRecords, setAuditRecords] = useState<AuditRecord[]>([]);
  const [versions, setVersions] = useState<ComponentVersion[]>([]);
  const [dependencies, setDependencies] = useState<ComponentDependency[]>([]);
  const [selectedVersionID, setSelectedVersionID] = useState("");
  const [actorID, setActorID] = useState("operator-1");
  const [decisionReason, setDecisionReason] = useState("Reviewed in BuildPlane console");
  const [canaryPercent, setCanaryPercent] = useState(10);
  const [candidateVersion, setCandidateVersion] = useState("v2");
  const [candidateSummary, setCandidateSummary] = useState("Adds explicit logistics and access keywords");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  const selectedVersion = useMemo(
    () => versions.find((version) => version.id === selectedVersionID) ?? versions[0] ?? null,
    [selectedVersionID, versions],
  );

  async function refresh() {
    setMessage("");
    const [healthBody, workflowRuns, componentVersions, affectedWorkflows] = await Promise.all([
      getHealth(),
      listWorkflowRuns(),
      listComponentVersions(),
      listAffectedWorkflows(),
    ]);
    setHealth(healthBody.status);
    setRuns(workflowRuns);
    setVersions(componentVersions);
    setDependencies(affectedWorkflows);
    setSelectedRunID((current) => current || workflowRuns[0]?.id || "");
    setSelectedVersionID((current) => current || componentVersions[0]?.id || "");
  }

  async function runAction(label: string, action: () => Promise<void>) {
    setBusy(true);
    setMessage("");
    try {
      await action();
      setMessage(label);
      await refresh();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Request failed");
    } finally {
      setBusy(false);
    }
  }

  useEffect(() => {
    refresh().catch((error: unknown) => {
      setHealth("unavailable");
      setMessage(error instanceof Error ? error.message : "Unable to load dashboard data");
    });
  }, []);

  useEffect(() => {
    if (!selectedRunID) {
      setSelectedRun(null);
      setAuditRecords([]);
      return;
    }

    const run = runs.find((candidate) => candidate.id === selectedRunID) ?? null;
    setSelectedRun(run);
    getWorkflowAudit(selectedRunID)
      .then(setAuditRecords)
      .catch((error: unknown) => setMessage(error instanceof Error ? error.message : "Unable to load audit records"));

    const source = new EventSource(eventSourceURL(`/v1/workflow-runs/${selectedRunID}/events`));
    source.addEventListener("workflow_run", (event) => {
      const payload = JSON.parse((event as MessageEvent).data) as WorkflowRunEvent;
      setSelectedRun(payload.workflow_run);
      setAuditRecords(payload.audit_records);
      setRuns((current) =>
        current.map((candidate) => (candidate.id === payload.workflow_run.id ? payload.workflow_run : candidate)),
      );
    });
    source.onerror = () => {
      source.close();
    };
    return () => source.close();
  }, [selectedRunID]);

  async function createDemoRun(workflowName: string, input: Record<string, unknown>) {
    await runAction("Workflow run created", async () => {
      const run = await createWorkflowRun(workflowName, input);
      setSelectedRunID(run.id);
    });
  }

  async function decide(decision: "approved" | "rejected") {
    if (!selectedRun) {
      return;
    }
    await runAction(`Decision ${decision}`, async () => {
      await submitHumanDecision(selectedRun.id, decision, actorID, decisionReason);
    });
  }

  async function createCandidate() {
    await runAction("Component candidate created", async () => {
      const version = await createComponentVersion(
        candidateVersion,
        `issue_classifier_${candidateVersion}`,
        candidateSummary,
        candidateSpec,
      );
      setSelectedVersionID(version.id);
    });
  }

  async function releaseAction(action: "evaluate" | "canary" | "promote" | "rollback") {
    if (!selectedVersion && action !== "rollback") {
      return;
    }
    await runAction(`Release action ${action} completed`, async () => {
      if (action === "evaluate" && selectedVersion) {
        await runEvaluation(selectedVersion.id);
      }
      if (action === "canary" && selectedVersion) {
        await startCanary(selectedVersion.id, canaryPercent);
      }
      if (action === "promote" && selectedVersion) {
        await promoteVersion(selectedVersion.id);
      }
      if (action === "rollback") {
        await rollbackIssueClassifier("Rollback requested from BuildPlane console");
      }
    });
  }

  return (
    <main className="shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">BuildPlane</p>
          <h1>Operations Console</h1>
        </div>
        <div className="topbar-actions">
          <span className={`status-pill ${health === "ok" ? "tone-good" : "tone-warn"}`}>
            <Activity size={16} aria-hidden="true" />
            API {health}
          </span>
          <button type="button" className="icon-button" onClick={() => void refresh()} aria-label="Refresh dashboard">
            <RefreshCw size={18} aria-hidden="true" />
          </button>
        </div>
      </header>

      <nav className="tabs" aria-label="Dashboard sections">
        <button type="button" className={tab === "workflows" ? "active" : ""} onClick={() => setTab("workflows")}>
          <Activity size={16} aria-hidden="true" />
          Workflows
        </button>
        <button type="button" className={tab === "components" ? "active" : ""} onClick={() => setTab("components")}>
          <GitBranch size={16} aria-hidden="true" />
          Components
        </button>
      </nav>

      {message ? <div className="notice">{message}</div> : null}

      {tab === "workflows" ? (
        <section className="workspace workflow-grid" aria-label="Workflow operations">
          <aside className="panel run-list">
            <div className="panel-header">
              <h2>Runs</h2>
              <span>{runs.length}</span>
            </div>
            <div className="button-row">
              <button type="button" onClick={() => void createDemoRun("demo.invoice-exception", invoiceDemoInput)} disabled={busy}>
                <Play size={16} aria-hidden="true" />
                Invoice
              </button>
              <button type="button" onClick={() => void createDemoRun("demo.freight-exception", freightDemoInput)} disabled={busy}>
                <Play size={16} aria-hidden="true" />
                Freight
              </button>
            </div>
            <div className="list">
              {runs.map((run) => (
                <button
                  type="button"
                  className={`list-item ${run.id === selectedRunID ? "selected" : ""}`}
                  key={run.id}
                  onClick={() => setSelectedRunID(run.id)}
                >
                  <span className="item-main">{run.workflow_name}</span>
                  <span className={`status-pill ${statusTone(run.status)}`}>{statusLabel(run.status)}</span>
                  <span className="muted">{compactID(run.id)}</span>
                </button>
              ))}
            </div>
          </aside>

          <section className="panel detail-panel">
            <div className="panel-header">
              <h2>{selectedRun?.workflow_name ?? "No workflow selected"}</h2>
              {selectedRun ? <span className={`status-pill ${statusTone(selectedRun.status)}`}>{statusLabel(selectedRun.status)}</span> : null}
            </div>
            {selectedRun ? (
              <div className="detail-grid">
                <div className="field-grid">
                  <label>
                    Actor
                    <input value={actorID} onChange={(event) => setActorID(event.target.value)} />
                  </label>
                  <label>
                    Reason
                    <input value={decisionReason} onChange={(event) => setDecisionReason(event.target.value)} />
                  </label>
                </div>
                <div className="button-row">
                  <button type="button" onClick={() => void decide("approved")} disabled={busy || selectedRun.status !== "waiting_for_human"}>
                    <CheckCircle2 size={16} aria-hidden="true" />
                    Approve
                  </button>
                  <button type="button" className="danger" onClick={() => void decide("rejected")} disabled={busy || selectedRun.status !== "waiting_for_human"}>
                    <XCircle size={16} aria-hidden="true" />
                    Reject
                  </button>
                </div>
                <div className="split">
                  <section className="subpanel">
                    <h3>Input</h3>
                    <pre>{formatJSON(selectedRun.input)}</pre>
                  </section>
                  <section className="subpanel">
                    <h3>Audit</h3>
                    <ol className="timeline">
                      {auditRecords.map((record) => (
                        <li key={record.id}>
                          <span className="timeline-dot" />
                          <strong>{record.event_type}</strong>
                          <span className="muted">{record.actor_type} / {record.actor_id || "system"}</span>
                          <code>{formatJSON(record.details)}</code>
                        </li>
                      ))}
                    </ol>
                  </section>
                </div>
              </div>
            ) : (
              <p className="empty">Start a synthetic workflow to populate the console.</p>
            )}
          </section>
        </section>
      ) : (
        <section className="workspace component-grid" aria-label="Component release operations">
          <section className="panel">
            <div className="panel-header">
              <h2>issue_classifier</h2>
              <span>{versions.length} versions</span>
            </div>
            <div className="field-grid">
              <label>
                Version
                <input value={candidateVersion} onChange={(event) => setCandidateVersion(event.target.value)} />
              </label>
              <label>
                Summary
                <input value={candidateSummary} onChange={(event) => setCandidateSummary(event.target.value)} />
              </label>
            </div>
            <div className="button-row">
              <button type="button" onClick={() => void createCandidate()} disabled={busy}>
                <ShieldCheck size={16} aria-hidden="true" />
                Candidate
              </button>
              <button type="button" onClick={() => void releaseAction("evaluate")} disabled={busy || !selectedVersion}>
                <FlaskConical size={16} aria-hidden="true" />
                Evaluate
              </button>
              <label className="inline-control">
                Canary
                <input
                  type="number"
                  min="1"
                  max="50"
                  value={canaryPercent}
                  onChange={(event) => setCanaryPercent(Number(event.target.value))}
                />
              </label>
              <button type="button" onClick={() => void releaseAction("canary")} disabled={busy || !selectedVersion}>
                <Activity size={16} aria-hidden="true" />
                Start
              </button>
              <button type="button" onClick={() => void releaseAction("promote")} disabled={busy || !selectedVersion}>
                <Rocket size={16} aria-hidden="true" />
                Promote
              </button>
              <button type="button" className="danger" onClick={() => void releaseAction("rollback")} disabled={busy}>
                <RotateCcw size={16} aria-hidden="true" />
                Rollback
              </button>
            </div>
            <div className="version-table" role="table" aria-label="Component versions">
              <div className="table-row table-head" role="row">
                <span>Version</span>
                <span>Status</span>
                <span>Eval</span>
                <span>Canary</span>
              </div>
              {versions.map((version) => (
                <button
                  type="button"
                  className={`table-row ${version.id === selectedVersionID ? "selected" : ""}`}
                  key={version.id}
                  onClick={() => setSelectedVersionID(version.id)}
                  role="row"
                >
                  <span>{version.version}</span>
                  <span className={`status-pill ${statusTone(version.status)}`}>{statusLabel(version.status)}</span>
                  <span>{version.evaluation_passed ? "passed" : "pending"}</span>
                  <span>{version.canary_percent}%</span>
                </button>
              ))}
            </div>
          </section>

          <section className="panel">
            <div className="panel-header">
              <h2>Dependencies</h2>
              <GitBranch size={18} aria-hidden="true" />
            </div>
            <div className="dependency-list">
              {dependencies.map((dependency) => (
                <div className="dependency" key={`${dependency.workflow_name}-${dependency.node_name}`}>
                  <strong>{dependency.workflow_name}</strong>
                  <span>{dependency.node_name}</span>
                  <p>{dependency.component_name}</p>
                </div>
              ))}
            </div>
            {selectedVersion ? (
              <section className="subpanel">
                <h3>{selectedVersion.version} spec</h3>
                <pre>{formatJSON(selectedVersion.spec)}</pre>
              </section>
            ) : null}
          </section>
        </section>
      )}
    </main>
  );
}
