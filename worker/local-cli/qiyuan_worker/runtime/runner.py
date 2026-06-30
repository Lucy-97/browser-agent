from __future__ import annotations

from qiyuan_worker.adapters.registry import AdapterRegistry, AdapterResolutionError
from qiyuan_worker.artifacts import ArtifactCollector
from qiyuan_worker.config import WorkerConfig
from qiyuan_worker.http_client import APIClient
from qiyuan_worker.manifest import append_checkpoint, job_dir, write_upload_manifest
from qiyuan_worker.protocols import AdapterResult, AutomationJob
from qiyuan_worker.runtime.context import JobContext
from qiyuan_worker.runtime.policy import PolicyViolation, validate_job_policy


class JobRunner:
    def __init__(
        self,
        registry: AdapterRegistry,
        capabilities: set[str],
    ):
        self.registry = registry
        self.capabilities = capabilities

    async def run(self, client: APIClient, config: WorkerConfig, job: AutomationJob) -> AdapterResult:
        work_dir = job_dir(config, job.job_id)
        collector = ArtifactCollector(work_dir / "artifacts")
        context = JobContext(
            job=job,
            config=config,
            api_client=client,
            artifact_collector=collector,
            work_dir=work_dir,
        )

        try:
            validate_job_policy(job)
            adapter = self.registry.resolve(job, self.capabilities)
            client.run_heartbeat(
                job.run_id,
                {
                    "status": "running",
                    "current_step": "adapter.prepare",
                    "cursor": job.cursor,
                    "message": f"preparing {adapter.name}",
                },
            )
            await adapter.prepare(context)
            client.run_heartbeat(
                job.run_id,
                {
                    "status": "running",
                    "current_step": "adapter.run",
                    "cursor": job.cursor,
                    "message": f"running {adapter.name}",
                },
            )
            result = await adapter.run(context)
        except PolicyViolation as exc:
            result = AdapterResult.failed(exc.code, exc.message, retryable=False)
        except AdapterResolutionError as exc:
            result = AdapterResult.failed(exc.code, exc.message, retryable=False)
        except Exception as exc:
            result = AdapterResult.failed("ADAPTER_RUNTIME_ERROR", str(exc), retryable=True)
        finally:
            if "adapter" in locals():
                try:
                    await adapter.cleanup(context)
                except Exception:
                    pass

        all_artifacts = (*collector.collected(), *result.artifacts)
        checkpoint_payload = {
            "job_id": job.job_id,
            "run_id": job.run_id,
            "cursor": result.cursor,
            "summary": result.summary,
            "status": result.status,
        }
        checkpoint_response = client.run_checkpoint(job.run_id, checkpoint_payload)
        append_checkpoint(
            config,
            job.job_id,
            {"request": checkpoint_payload, "response": checkpoint_response},
        )

        uploaded_artifacts = []
        for artifact in all_artifacts:
            if artifact.local_path and artifact.local_path.exists():
                uploaded_artifacts.append(
                    client.upload_run_artifact_file(
                        job.run_id,
                        artifact.artifact_type,
                        artifact.local_path,
                        metadata=artifact.metadata,
                    )
                )
            else:
                payload = artifact.to_payload()
                uploaded_artifacts.append(client.create_run_artifact(job.run_id, payload))

        write_upload_manifest(
            config,
            job.job_id,
            {
                "job_id": job.job_id,
                "run_id": job.run_id,
                "artifacts": uploaded_artifacts,
                "checkpoint": checkpoint_payload,
            },
        )

        if result.manual_action:
            client.create_manual_action(job.run_id, result.manual_action.to_payload())

        client.complete_run(
            job.run_id,
            {
                "job_id": job.job_id,
                "status": result.status,
                "summary": result.summary,
                "last_cursor": result.cursor,
                "error": {
                    "code": result.error_code,
                    "message": result.error_message,
                    "retryable": result.retryable,
                }
                if result.error_code
                else None,
            },
        )
        return result
