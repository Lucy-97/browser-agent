package automation

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	automationmodel "github.com/Lucy-97/browser-agent/backend-api/internal/model/automation"
	workermodel "github.com/Lucy-97/browser-agent/backend-api/internal/model/worker"
)

type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (repo *MySQLRepository) CreateJob(req automationmodel.CreateJobRequest) automationmodel.Job {
	now := time.Now().UTC()
	job := automationmodel.Job{
		ID:        mysqlNewID("job"),
		Type:      req.JobType,
		Adapter:   req.Adapter,
		Target:    mapOrEmpty(req.Target),
		Input:     mapOrEmpty(req.Input),
		Policy:    mapOrEmpty(req.Policy),
		Status:    "queued",
		Priority:  req.Priority,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := repo.db.ExecContext(
		context.Background(),
		`INSERT INTO automation_job (
			id, job_type, adapter, status, priority, target_json, input_json, policy_json, created_at, updated_at
		) VALUES (?, ?, ?, 'queued', ?, ?, ?, ?, ?, ?)`,
		job.ID,
		job.Type,
		job.Adapter,
		job.Priority,
		mustJSON(job.Target),
		mustJSON(job.Input),
		mustJSON(job.Policy),
		now,
		now,
	)
	if err != nil {
		panic(err)
	}
	return job
}

func (repo *MySQLRepository) Job(jobID string) (automationmodel.Job, error) {
	row := repo.db.QueryRowContext(
		context.Background(),
		`SELECT id, job_type, adapter, status, priority, target_json, input_json, policy_json,
		        last_cursor_json, created_at, updated_at
		 FROM automation_job
		 WHERE id = ?`,
		jobID,
	)
	return scanJob(row)
}

func (repo *MySQLRepository) ListJobs(opts automationmodel.ListJobsOptions) ([]automationmodel.Job, error) {
	where := []string{"1 = 1"}
	args := make([]any, 0)
	if opts.Status != "" {
		where = append(where, "status = ?")
		args = append(args, opts.Status)
	}
	if opts.Adapter != "" {
		where = append(where, "adapter = ?")
		args = append(args, opts.Adapter)
	}
	args = append(args, opts.Limit, opts.Offset)
	rows, err := repo.db.QueryContext(
		context.Background(),
		`SELECT id, job_type, adapter, status, priority, target_json, input_json, policy_json,
		        last_cursor_json, created_at, updated_at
		 FROM automation_job
		 WHERE `+strings.Join(where, " AND ")+`
		 ORDER BY created_at DESC
		 LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]automationmodel.Job, 0)
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (repo *MySQLRepository) NextJob(device workermodel.Device) (automationmodel.JobEnvelope, error) {
	tx, err := repo.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return automationmodel.JobEnvelope{}, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(
		context.Background(),
		`SELECT id, job_type, adapter, target_json, input_json, policy_json, last_cursor_json
		 FROM automation_job
		 WHERE status = 'queued'
		 ORDER BY priority DESC, created_at ASC
		 LIMIT 20
		 FOR UPDATE`,
	)
	if err != nil {
		return automationmodel.JobEnvelope{}, err
	}
	defer rows.Close()

	var selected *automationmodel.JobEnvelope
	for rows.Next() {
		var envelope automationmodel.JobEnvelope
		var targetRaw, inputRaw, policyRaw []byte
		var cursorRaw sql.NullString
		if err := rows.Scan(
			&envelope.JobID,
			&envelope.JobType,
			&envelope.Adapter,
			&targetRaw,
			&inputRaw,
			&policyRaw,
			&cursorRaw,
		); err != nil {
			return automationmodel.JobEnvelope{}, err
		}
		if !adapterAllowed(envelope.Adapter, device.Capabilities) {
			continue
		}
		envelope.Target = decodeMap(targetRaw)
		envelope.Input = decodeMap(inputRaw)
		envelope.Policy = decodeMap(policyRaw)
		envelope.Cursor = decodeNullableMap(cursorRaw)
		selected = &envelope
		break
	}
	if err := rows.Err(); err != nil {
		return automationmodel.JobEnvelope{}, err
	}
	if err := rows.Close(); err != nil {
		return automationmodel.JobEnvelope{}, err
	}
	if selected == nil {
		return automationmodel.JobEnvelope{}, ErrNoJobAvailable
	}

	now := time.Now().UTC()
	runID := mysqlNewID("run")
	_, err = tx.ExecContext(
		context.Background(),
		`INSERT INTO automation_run (
			id, job_id, device_id, adapter, status, worker_version, capabilities_json, started_at
		) VALUES (?, ?, ?, ?, 'running', ?, ?, ?)`,
		runID,
		selected.JobID,
		device.ID,
		selected.Adapter,
		device.WorkerVersion,
		mustJSON(device.Capabilities),
		now,
	)
	if err != nil {
		return automationmodel.JobEnvelope{}, err
	}
	_, err = tx.ExecContext(
		context.Background(),
		`UPDATE automation_job
		 SET status = 'running', assigned_device_id = ?, assigned_at = ?, started_at = ?, updated_at = ?
		 WHERE id = ? AND status = 'queued'`,
		device.ID,
		now,
		now,
		now,
		selected.JobID,
	)
	if err != nil {
		return automationmodel.JobEnvelope{}, err
	}
	if err := tx.Commit(); err != nil {
		return automationmodel.JobEnvelope{}, err
	}
	selected.RunID = runID
	return *selected, nil
}

func (repo *MySQLRepository) Run(runID string) (automationmodel.Run, error) {
	row := repo.db.QueryRowContext(
		context.Background(),
		`SELECT id, job_id, device_id, status, started_at, last_heartbeat_at, ended_at,
		        summary_json, error_code, error_message
		 FROM automation_run
		 WHERE id = ?`,
		runID,
	)
	return scanRun(row)
}

func (repo *MySQLRepository) ListRuns(opts automationmodel.ListRunsOptions) ([]automationmodel.Run, error) {
	where := []string{"1 = 1"}
	args := make([]any, 0)
	if opts.Status != "" {
		where = append(where, "status = ?")
		args = append(args, opts.Status)
	}
	if opts.JobID != "" {
		where = append(where, "job_id = ?")
		args = append(args, opts.JobID)
	}
	if opts.DeviceID != "" {
		where = append(where, "device_id = ?")
		args = append(args, opts.DeviceID)
	}
	args = append(args, opts.Limit, opts.Offset)
	rows, err := repo.db.QueryContext(
		context.Background(),
		`SELECT id, job_id, device_id, status, started_at, last_heartbeat_at, ended_at,
		        summary_json, error_code, error_message
		 FROM automation_run
		 WHERE `+strings.Join(where, " AND ")+`
		 ORDER BY started_at DESC
		 LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := make([]automationmodel.Run, 0)
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (repo *MySQLRepository) Heartbeat(runID string, req automationmodel.HeartbeatRequest) (automationmodel.Run, error) {
	existing, err := repo.Run(runID)
	if err != nil {
		return automationmodel.Run{}, err
	}
	if existing.Status == "cancelled" {
		return existing, nil
	}
	now := time.Now().UTC()
	_, err = repo.db.ExecContext(
		context.Background(),
		`UPDATE automation_run
		 SET status = ?, last_heartbeat_at = ?
		 WHERE id = ?`,
		req.Status,
		now,
		runID,
	)
	if err != nil {
		return automationmodel.Run{}, err
	}
	return repo.Run(runID)
}

func (repo *MySQLRepository) Checkpoint(runID string, checkpoint automationmodel.Checkpoint) (automationmodel.Checkpoint, error) {
	run, err := repo.Run(runID)
	if err != nil {
		return automationmodel.Checkpoint{}, err
	}
	sequence := 1
	if err := repo.db.QueryRowContext(
		context.Background(),
		`SELECT COALESCE(MAX(sequence), 0) + 1 FROM automation_checkpoint WHERE run_id = ?`,
		runID,
	).Scan(&sequence); err != nil {
		return automationmodel.Checkpoint{}, err
	}
	stored := automationmodel.Checkpoint{
		ID:        mysqlNewID("chk"),
		RunID:     runID,
		JobID:     run.JobID,
		Cursor:    mapOrEmpty(checkpoint.Cursor),
		Summary:   mapOrEmpty(checkpoint.Summary),
		Status:    checkpoint.Status,
		CreatedAt: time.Now().UTC(),
	}
	_, err = repo.db.ExecContext(
		context.Background(),
		`INSERT INTO automation_checkpoint (
			id, job_id, run_id, sequence, stage, cursor_json, progress_json, result_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, JSON_OBJECT(), ?, ?)`,
		stored.ID,
		stored.JobID,
		stored.RunID,
		sequence,
		stored.Status,
		mustJSON(stored.Cursor),
		mustJSON(stored.Summary),
		stored.CreatedAt,
	)
	return stored, err
}

func (repo *MySQLRepository) Checkpoints(runID string) ([]automationmodel.Checkpoint, error) {
	if _, err := repo.Run(runID); err != nil {
		return nil, err
	}
	rows, err := repo.db.QueryContext(
		context.Background(),
		`SELECT id, job_id, run_id, stage, cursor_json, result_json, created_at
		 FROM automation_checkpoint
		 WHERE run_id = ?
		 ORDER BY sequence ASC, created_at ASC`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	checkpoints := make([]automationmodel.Checkpoint, 0)
	for rows.Next() {
		var checkpoint automationmodel.Checkpoint
		var cursorRaw, summaryRaw []byte
		if err := rows.Scan(
			&checkpoint.ID,
			&checkpoint.JobID,
			&checkpoint.RunID,
			&checkpoint.Status,
			&cursorRaw,
			&summaryRaw,
			&checkpoint.CreatedAt,
		); err != nil {
			return nil, err
		}
		checkpoint.Cursor = decodeMap(cursorRaw)
		checkpoint.Summary = decodeMap(summaryRaw)
		checkpoints = append(checkpoints, checkpoint)
	}
	return checkpoints, rows.Err()
}

func (repo *MySQLRepository) CreateArtifact(runID string, artifact automationmodel.Artifact) (automationmodel.Artifact, error) {
	run, err := repo.Run(runID)
	if err != nil {
		return automationmodel.Artifact{}, err
	}
	artifact.ID = mysqlNewID("art")
	artifact.RunID = runID
	artifact.CreatedAt = time.Now().UTC()
	_, err = repo.db.ExecContext(
		context.Background(),
		`INSERT INTO automation_artifact (
			id, job_id, run_id, artifact_type, storage_key, filename, content_type,
			size_bytes, sha256, metadata_json, redaction_status, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'not_required', ?)`,
		artifact.ID,
		run.JobID,
		runID,
		artifact.ArtifactType,
		artifact.LocalPath,
		"",
		"",
		artifact.SizeBytes,
		artifact.SHA256,
		mustJSON(mapOrEmpty(artifact.Metadata)),
		artifact.CreatedAt,
	)
	return artifact, err
}

func (repo *MySQLRepository) Artifacts(runID string) ([]automationmodel.Artifact, error) {
	rows, err := repo.db.QueryContext(
		context.Background(),
		`SELECT id, run_id, artifact_type, storage_key, size_bytes, sha256, metadata_json, created_at
		 FROM automation_artifact
		 WHERE run_id = ?
		 ORDER BY created_at ASC`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	artifacts := make([]automationmodel.Artifact, 0)
	for rows.Next() {
		var artifact automationmodel.Artifact
		var localPath sql.NullString
		var sha sql.NullString
		var size sql.NullInt64
		var metadataRaw []byte
		if err := rows.Scan(
			&artifact.ID,
			&artifact.RunID,
			&artifact.ArtifactType,
			&localPath,
			&size,
			&sha,
			&metadataRaw,
			&artifact.CreatedAt,
		); err != nil {
			return nil, err
		}
		artifact.LocalPath = localPath.String
		artifact.SHA256 = sha.String
		if size.Valid {
			artifact.SizeBytes = &size.Int64
		}
		artifact.Metadata = decodeMap(metadataRaw)
		artifacts = append(artifacts, artifact)
	}
	return artifacts, rows.Err()
}

func (repo *MySQLRepository) Artifact(artifactID string) (automationmodel.Artifact, error) {
	row := repo.db.QueryRowContext(
		context.Background(),
		`SELECT id, run_id, artifact_type, storage_key, size_bytes, sha256, metadata_json, created_at
		 FROM automation_artifact
		 WHERE id = ?`,
		artifactID,
	)
	artifact, err := scanArtifact(row)
	if errors.Is(err, sql.ErrNoRows) {
		return automationmodel.Artifact{}, ErrArtifactNotFound
	}
	return artifact, err
}

func (repo *MySQLRepository) CreateManualAction(runID string, action automationmodel.ManualAction) (automationmodel.ManualAction, error) {
	run, err := repo.Run(runID)
	if err != nil {
		return automationmodel.ManualAction{}, err
	}
	action.ID = mysqlNewID("act")
	action.RunID = runID
	action.Status = "pending"
	action.CreatedAt = time.Now().UTC()
	_, err = repo.db.ExecContext(
		context.Background(),
		`INSERT INTO automation_manual_action (
			id, job_id, run_id, type, status, prompt, details_json, created_at
		) VALUES (?, ?, ?, ?, 'pending', ?, ?, ?)`,
		action.ID,
		run.JobID,
		runID,
		action.ActionType,
		action.Message,
		mustJSON(mapOrEmpty(action.Payload)),
		action.CreatedAt,
	)
	return action, err
}

func (repo *MySQLRepository) ManualActions(runID string) ([]automationmodel.ManualAction, error) {
	rows, err := repo.db.QueryContext(
		context.Background(),
		`SELECT id, run_id, type, status, prompt, details_json, created_at, resolved_at
		 FROM automation_manual_action
		 WHERE run_id = ?
		 ORDER BY created_at ASC`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	actions := make([]automationmodel.ManualAction, 0)
	for rows.Next() {
		action, err := scanManualAction(rows)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	return actions, rows.Err()
}

func (repo *MySQLRepository) ListManualActions(opts automationmodel.ListManualActionsOptions) ([]automationmodel.ManualAction, error) {
	where := []string{"1 = 1"}
	args := make([]any, 0)
	if opts.Status != "" {
		where = append(where, "status = ?")
		args = append(args, opts.Status)
	}
	if opts.RunID != "" {
		where = append(where, "run_id = ?")
		args = append(args, opts.RunID)
	}
	args = append(args, opts.Limit, opts.Offset)
	rows, err := repo.db.QueryContext(
		context.Background(),
		`SELECT id, run_id, type, status, prompt, details_json, created_at, resolved_at
		 FROM automation_manual_action
		 WHERE `+strings.Join(where, " AND ")+`
		 ORDER BY created_at DESC
		 LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	actions := make([]automationmodel.ManualAction, 0)
	for rows.Next() {
		action, err := scanManualAction(rows)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	return actions, rows.Err()
}

func (repo *MySQLRepository) ManualAction(actionID string) (automationmodel.ManualAction, error) {
	row := repo.db.QueryRowContext(
		context.Background(),
		`SELECT id, run_id, type, status, prompt, details_json, created_at, resolved_at
		 FROM automation_manual_action
		 WHERE id = ?`,
		actionID,
	)
	action, err := scanManualAction(row)
	if errors.Is(err, sql.ErrNoRows) {
		return automationmodel.ManualAction{}, ErrManualActionNotFound
	}
	return action, err
}

func (repo *MySQLRepository) ResolveManualAction(actionID string, req automationmodel.ResolveManualActionRequest) (automationmodel.ManualAction, error) {
	now := time.Now().UTC()
	result, err := repo.db.ExecContext(
		context.Background(),
		`UPDATE automation_manual_action
		 SET status = ?, details_json = ?, resolved_at = ?
		 WHERE id = ?`,
		req.Status,
		mustJSON(mapOrEmpty(req.Payload)),
		now,
		actionID,
	)
	if err != nil {
		return automationmodel.ManualAction{}, err
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return automationmodel.ManualAction{}, ErrManualActionNotFound
	}
	return repo.ManualAction(actionID)
}

func (repo *MySQLRepository) CompleteRun(runID string, req automationmodel.CompleteRunRequest) (automationmodel.Run, error) {
	run, err := repo.Run(runID)
	if err != nil {
		return automationmodel.Run{}, err
	}
	if run.Status == "cancelled" {
		return run, nil
	}
	now := time.Now().UTC()
	errorCode, errorMessage := errorFields(req.Error)
	_, err = repo.db.ExecContext(
		context.Background(),
		`UPDATE automation_run
		 SET status = ?, ended_at = ?, summary_json = ?, error_code = ?, error_message = ?
		 WHERE id = ?`,
		req.Status,
		now,
		mustJSON(mapOrEmpty(req.Summary)),
		nullString(errorCode),
		nullString(errorMessage),
		runID,
	)
	if err != nil {
		return automationmodel.Run{}, err
	}
	_, err = repo.db.ExecContext(
		context.Background(),
		`UPDATE automation_job
		 SET status = ?, last_cursor_json = ?, last_error_code = ?, last_error_message = ?,
		     completed_at = ?, updated_at = ?
		 WHERE id = ?`,
		req.Status,
		mustJSON(mapOrEmpty(req.LastCursor)),
		nullString(errorCode),
		nullString(errorMessage),
		now,
		now,
		run.JobID,
	)
	if err != nil {
		return automationmodel.Run{}, err
	}
	return repo.Run(runID)
}

func (repo *MySQLRepository) CancelRun(runID string, reason string) (automationmodel.Run, error) {
	run, err := repo.Run(runID)
	if err != nil {
		return automationmodel.Run{}, err
	}
	now := time.Now().UTC()
	_, err = repo.db.ExecContext(
		context.Background(),
		`UPDATE automation_run
		 SET status = 'cancelled', ended_at = ?, error_code = 'RUN_CANCELLED', error_message = ?
		 WHERE id = ?`,
		now,
		reason,
		runID,
	)
	if err != nil {
		return automationmodel.Run{}, err
	}
	_, err = repo.db.ExecContext(
		context.Background(),
		`UPDATE automation_job
		 SET status = 'cancelled', last_error_code = 'RUN_CANCELLED', last_error_message = ?, completed_at = ?, updated_at = ?
		 WHERE id = ?`,
		reason,
		now,
		now,
		run.JobID,
	)
	if err != nil {
		return automationmodel.Run{}, err
	}
	return repo.Run(runID)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanJob(row scanner) (automationmodel.Job, error) {
	var job automationmodel.Job
	var targetRaw, inputRaw, policyRaw []byte
	var cursorRaw sql.NullString
	err := row.Scan(
		&job.ID,
		&job.Type,
		&job.Adapter,
		&job.Status,
		&job.Priority,
		&targetRaw,
		&inputRaw,
		&policyRaw,
		&cursorRaw,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return automationmodel.Job{}, ErrJobNotFound
	}
	if err != nil {
		return automationmodel.Job{}, err
	}
	job.Target = decodeMap(targetRaw)
	job.Input = decodeMap(inputRaw)
	job.Policy = decodeMap(policyRaw)
	job.Cursor = decodeNullableMap(cursorRaw)
	return job, nil
}

func scanRun(row scanner) (automationmodel.Run, error) {
	var run automationmodel.Run
	var summaryRaw sql.NullString
	var errorCode sql.NullString
	var errorMessage sql.NullString
	var lastHeartbeat sql.NullTime
	var endedAt sql.NullTime
	err := row.Scan(
		&run.ID,
		&run.JobID,
		&run.DeviceID,
		&run.Status,
		&run.StartedAt,
		&lastHeartbeat,
		&endedAt,
		&summaryRaw,
		&errorCode,
		&errorMessage,
	)
	if err == sql.ErrNoRows {
		return automationmodel.Run{}, ErrRunNotFound
	}
	if err != nil {
		return automationmodel.Run{}, err
	}
	if lastHeartbeat.Valid {
		run.LastHeartbeatAt = &lastHeartbeat.Time
	}
	if endedAt.Valid {
		run.CompletedAt = &endedAt.Time
	}
	run.Summary = decodeNullableMap(summaryRaw)
	if errorCode.Valid || errorMessage.Valid {
		run.Error = map[string]any{"code": errorCode.String, "message": errorMessage.String}
	}
	return run, nil
}

func scanArtifact(row scanner) (automationmodel.Artifact, error) {
	var artifact automationmodel.Artifact
	var localPath sql.NullString
	var sha sql.NullString
	var size sql.NullInt64
	var metadataRaw []byte
	err := row.Scan(
		&artifact.ID,
		&artifact.RunID,
		&artifact.ArtifactType,
		&localPath,
		&size,
		&sha,
		&metadataRaw,
		&artifact.CreatedAt,
	)
	if err != nil {
		return automationmodel.Artifact{}, err
	}
	artifact.LocalPath = localPath.String
	artifact.SHA256 = sha.String
	if size.Valid {
		artifact.SizeBytes = &size.Int64
	}
	artifact.Metadata = decodeMap(metadataRaw)
	return artifact, nil
}

func scanManualAction(row scanner) (automationmodel.ManualAction, error) {
	var action automationmodel.ManualAction
	var payloadRaw []byte
	var resolvedAt sql.NullTime
	err := row.Scan(
		&action.ID,
		&action.RunID,
		&action.ActionType,
		&action.Status,
		&action.Message,
		&payloadRaw,
		&action.CreatedAt,
		&resolvedAt,
	)
	if err != nil {
		return automationmodel.ManualAction{}, err
	}
	action.Payload = decodeMap(payloadRaw)
	if resolvedAt.Valid {
		action.ResolvedAt = &resolvedAt.Time
	}
	return action, nil
}

func decodeMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return map[string]any{}
	}
	return value
}

func decodeNullableMap(raw sql.NullString) map[string]any {
	if !raw.Valid {
		return map[string]any{}
	}
	return decodeMap([]byte(raw.String))
}

func mustJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func errorFields(value map[string]any) (string, string) {
	if value == nil {
		return "", ""
	}
	code, _ := value["code"].(string)
	message, _ := value["message"].(string)
	return code, message
}

func mysqlNewID(prefix string) string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(buf[:])
}
