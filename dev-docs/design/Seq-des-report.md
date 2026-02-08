# CI/CD System — Reporting Sequence Diagram

## Workflow Overview

This diagram illustrates how users query pipeline execution results through the reporting system. The Reporting Service aggregates data from PostgreSQL and returns formatted results based on user-specified filters.

```mermaid
sequenceDiagram
    actor Dev as Developer
    participant CLI as CLI
    participant GW as Gateway<br/>:8000
    participant Report as Report<br/>:8004
    participant DB as PostgreSQL<br/>:5432

    Dev->>CLI: cicd report --pipeline default<br/>--run 1 --stage build
    
    CLI->>GW: GET /report?pipeline=default<br/>&run=1&stage=build
    
    GW->>Report: GET /report<br/>{pipeline, run, stage}
    
    Report->>DB: SELECT * FROM pipeline_runs<br/>WHERE name='default' AND run_no=1
    
    alt Pipeline run exists
        DB-->>Report: Pipeline data<br/>(name, status, timestamps, git_info)
        
        Report->>DB: SELECT * FROM stages<br/>WHERE run_id=1 AND name='build'
        
        alt Stage exists
            DB-->>Report: Stage data<br/>(status, timestamps)
            
            Report->>DB: SELECT * FROM jobs<br/>WHERE stage_id=101
            DB-->>Report: Jobs list<br/>(compile: success)
            
            Report->>Report: Format as JSON
            Report-->>GW: JSON report
            GW-->>CLI: Report data
            
            CLI-->>Dev: (Formatted JSON to YAML response)<br/>pipeline:<br/>  name: default<br/>  run-no: 1<br/>  stage:<br/>    name: build<br/>    jobs:<br/>      - compile: success
            
        else Stage not found
            Report-->>GW: 404 Stage not found
            GW-->>CLI: Error response
            CLI-->>Dev: ✗ Stage 'build' not found
        end
        
    else Pipeline run not found
        Report-->>GW: 404 Pipeline not found
        GW-->>CLI: Error response
        CLI-->>Dev: ✗ No run #1 for pipeline 'default'
    end

```

### Key Workflow Phases

1. **Report Request** (Steps 1-3)
    - Developer runs `cicd report` command with filters (pipeline name, run number, stage name)
    - CLI constructs query parameters and sends GET request to API Gateway
    - Gateway routes the request to Reporting Service

2. **Pipeline Run Lookup** (Steps 4-5)
    - Reporting Service queries PostgreSQL for the specified pipeline run
    - Two possible outcomes:
        - **Run exists**: Proceed to stage lookup
        - **Run not found**: Return 404 error immediately

3. **Stage Lookup** (Steps 6-7, if pipeline run exists)
    - Reporting Service queries for the specified stage within the pipeline run
    - Two possible outcomes:
        - **Stage exists**: Proceed to job aggregation
        - **Stage not found**: Return 404 error

4. **Job Aggregation** (Steps 8-9, if stage exists)
    - Reporting Service fetches all jobs belonging to the stage
    - Database returns job details including name and execution status

5. **Response Formatting** (Steps 10-13)
    - Reporting Service formats aggregated data as JSON
    - Response flows back through Gateway to CLI
    - CLI converts JSON to human-readable YAML format
    - Developer sees structured report of pipeline execution results

### Query Flexibility

The Reporting Service supports various query patterns:

- **Full pipeline report**: `cicd report --pipeline default --run 1` (returns all stages and jobs)
- **Stage-specific report**: `cicd report --pipeline default --run 1 --stage build` (returns jobs in 'build' stage)
- **Latest run**: `cicd report --pipeline default` (omit run number to get most recent execution)