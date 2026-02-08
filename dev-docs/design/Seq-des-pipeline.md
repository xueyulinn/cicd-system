# Sequence Design for Pipeline Execution


## Workflow Overview

This diagram illustrates the complete execution flow of a CI/CD pipeline run, from user initiation to completion.
```mermaid
sequenceDiagram
    actor Dev as Developer
    participant CLI as CLI
    participant GW as Gateway<br/>:8000
    participant Val as Validation<br/>:8001
    participant Exec as Execution<br/>:8002
    participant Queue as Job Queue<br/>:5672
    participant Worker as Worker<br/>:8003
    participant DB as PostgreSQL<br/>:5432
    participant Docker as Docker

    Dev->>CLI: cicd run --file default.yaml
    CLI->>CLI: Read YAML from Git/Dir
    CLI->>GW: POST /run (yaml_content)

    GW->>Exec: POST /execute
    Exec->>Val: POST /validate
    Val-->>Exec: {valid: true}
    Val-->> GW: {valid: False}
    GW-->> CLI: {valid: False}
    CLI-->> Dev: ✗ Pipeline validation failed

    Exec->>DB: INSERT pipeline_run<br/>(status='running')
    DB-->>Exec: run_id=1

    rect rgb(240, 248, 255, 0)
        Note over Exec,Docker: Execute All Stages (build → test → deploy)

        loop For each stage
            Exec->>DB: INSERT stage
            DB-->>Exec: stage_id

            loop For each job in stage
                Exec->>DB: INSERT job
                DB-->>Exec: job_id

                Exec->>Queue: Enqueue job<br/>{job_id, image, script}
                Queue-->>Exec: Job enqueued

                Worker->>Queue: Consume job (blocking pull)
                Queue-->>Worker: {job_id, image, script}

                Worker->>Docker: Pull image & create container
                Docker-->>Worker: Container running

                loop Stream logs
                    Docker-->>Worker: Container logs
                    Worker->>DB: INSERT execution_logs
                end

                Docker-->>Worker: exit_code
                Worker->>Docker: Remove container

                Worker->>Queue: Publish status update<br/>{job_id, status, exit_code}
                Queue-->>Exec: Status event (via subscription)
                Exec->>DB: UPDATE job (status='success')
            end

            Exec->>DB: UPDATE stage (status='success')
        end
    end

    Exec->>DB: UPDATE pipeline_run<br/>(status='success')
    Exec-->>GW: {run_id: 1, status: 'success'}
    GW-->>CLI: Pipeline complete
    CLI-->>Dev: ✓ Pipeline run 1 succeeded

```

### Key Workflow Phases

1. **Pipeline Submission** (Steps 1-3)
    - Developer runs `cicd run` command via CLI
    - CLI reads pipeline YAML configuration from Git repository or local directory
    - CLI sends pipeline definition to API Gateway

2. **Validation** (Steps 4-7)
    - Gateway forwards request to Execution Service
    - Execution Service validates pipeline configuration via Validation Service
    - If validation fails, error is returned immediately to the user
    - If validation succeeds, pipeline execution begins

3. **Pipeline Initialization** (Steps 8-9)
    - Execution Service creates a new pipeline run record in PostgreSQL
    - Database returns a unique `run_id` for tracking

4. **Job Dispatch and Execution** (Steps 10-23, looped per stage/job)
    - For each stage in the pipeline (e.g., build → test → deploy):
        - Execution Service creates stage and job records in the database
        - Jobs are enqueued to the Job Queue (Redis/RabbitMQ) with execution details
        - Worker Service consumes jobs from the queue asynchronously
        - Worker pulls Docker image and creates container for job execution
        - Container logs are streamed and stored in PostgreSQL
        - Worker publishes job completion status back to the queue
        - Execution Service receives status updates via queue subscription and updates database

5. **Pipeline Completion** (Steps 24-27)
    - After all stages complete, Execution Service marks pipeline run as successful
    - Success status is propagated back through Gateway to CLI
    - Developer receives confirmation of successful pipeline execution

### Key Design Features

- **Asynchronous Processing**: Job Queue decouples job dispatch from execution, allowing Workers to process jobs at their own pace
- **Horizontal Scalability**: Multiple Worker instances can consume from the same queue for parallel job execution
- **State Persistence**: All execution state (runs, stages, jobs, logs) is persisted in PostgreSQL for auditing and reporting
- **Event-Driven Updates**: Status changes flow through the queue as events, enabling loose coupling between services