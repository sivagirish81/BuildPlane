CREATE TABLE component_versions (
	id text PRIMARY KEY,
	component_name text NOT NULL,
	version text NOT NULL,
	prompt_version text NOT NULL,
	spec jsonb NOT NULL,
	status text NOT NULL,
	change_summary text NOT NULL DEFAULT '',
	created_by text NOT NULL,
	canary_percent integer NOT NULL DEFAULT 0,
	evaluation_passed boolean NOT NULL DEFAULT false,
	previous_promoted_version_id text REFERENCES component_versions(id),
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT component_versions_status_check CHECK (
		status IN ('candidate', 'evaluated', 'canary', 'promoted', 'superseded', 'rolled_back')
	),
	CONSTRAINT component_versions_canary_percent_check CHECK (
		canary_percent >= 0 AND canary_percent <= 100
	),
	CONSTRAINT component_versions_unique_version UNIQUE (component_name, version)
);

CREATE INDEX component_versions_component_status_idx
	ON component_versions (component_name, status, updated_at);

CREATE TABLE component_evaluation_runs (
	id text PRIMARY KEY,
	component_version_id text NOT NULL REFERENCES component_versions(id) ON DELETE CASCADE,
	component_name text NOT NULL,
	baseline_version_id text NOT NULL REFERENCES component_versions(id),
	dataset_name text NOT NULL,
	status text NOT NULL,
	candidate_passed boolean NOT NULL,
	baseline_passed boolean NOT NULL,
	candidate_passed_cases integer NOT NULL,
	candidate_total_cases integer NOT NULL,
	baseline_passed_cases integer NOT NULL,
	baseline_total_cases integer NOT NULL,
	summary jsonb NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	CONSTRAINT component_evaluation_runs_status_check CHECK (
		status IN ('passed', 'failed')
	)
);

CREATE INDEX component_evaluation_runs_component_version_idx
	ON component_evaluation_runs (component_version_id, created_at);

CREATE TABLE component_evaluation_results (
	id bigserial PRIMARY KEY,
	evaluation_run_id text NOT NULL REFERENCES component_evaluation_runs(id) ON DELETE CASCADE,
	case_name text NOT NULL,
	workflow_name text NOT NULL,
	expected_category text NOT NULL,
	baseline_category text NOT NULL,
	candidate_category text NOT NULL,
	baseline_passed boolean NOT NULL,
	candidate_passed boolean NOT NULL,
	details jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE component_release_events (
	id bigserial PRIMARY KEY,
	component_name text NOT NULL,
	component_version_id text NOT NULL REFERENCES component_versions(id),
	event_type text NOT NULL,
	actor_id text NOT NULL,
	details jsonb NOT NULL DEFAULT '{}'::jsonb,
	created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX component_release_events_component_created_idx
	ON component_release_events (component_name, created_at);

INSERT INTO component_versions (
	id,
	component_name,
	version,
	prompt_version,
	spec,
	status,
	change_summary,
	created_by,
	evaluation_passed
) VALUES (
	'issue-classifier-v1',
	'issue_classifier',
	'v1',
	'issue_classifier_v1',
	'{
		"rules": [
			{"category": "billing", "keywords": ["invoice", "payment", "charge", "vendor"]},
			{"category": "logistics", "keywords": ["freight", "shipment", "carrier", "delivery"]},
			{"category": "access", "keywords": ["access", "login", "portal"]}
		],
		"default_category": "general"
	}'::jsonb,
	'promoted',
	'Initial synthetic issue classifier baseline',
	'system',
	true
);

INSERT INTO component_release_events (
	component_name,
	component_version_id,
	event_type,
	actor_id,
	details
) VALUES (
	'issue_classifier',
	'issue-classifier-v1',
	'component_version.seeded',
	'system',
	'{"status":"promoted"}'::jsonb
);
