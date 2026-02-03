# CI/CD High-Level Architecture

## Overview
This document describes the architecture of our custom CI/CD system.

## System Diagram
```mermaid
graph TD
    User((User))
    CLI[CLI]
    Coordinator["Coordinator"]
    Verifier[Verifier]
    Loader[Loader]
    Scheduler[Scheduler]
    Queue[("Queue")]
    Database[(Database)]
    Runner[Runner]

    User -->|interact| CLI
    CLI -->|/get /post| Coordinator
    
    Verifier -.->|validate| Coordinator
    Loader -.->|load config| Coordinator
    Scheduler -.->|schedule jobs| Coordinator

   Coordinator -->|write metadata| Database
   Coordinator -->|enqueue jobs| Queue
   Queue -->|dequeue jobs| Runner
   Runner -->|return results| Coordinator

    style User fill:#e1f5ff,stroke:#01579b,stroke-width:2px,color:#000
    style CLI fill:#fff3e0,stroke:#e65100,stroke-width:2px,color:#000
    style Coordinator fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px,color:#000
    style Verifier fill:#f3e5f5,stroke:#6a1b9a,stroke-width:2px,color:#000
    style Loader fill:#f3e5f5,stroke:#6a1b9a,stroke-width:2px,color:#000
    style Scheduler fill:#f3e5f5,stroke:#6a1b9a,stroke-width:2px,color:#000
    style Queue fill:#fff9c4,stroke:#f57f17,stroke-width:2px,color:#000
    style Database fill:#e0f2f1,stroke:#00695c,stroke-width:2px,color:#000
    style Runner fill:#fce4ec,stroke:#c2185b,stroke-width:2px,color:#000
```

## Components

- **CLI**: Command-line interface for user interaction
- **Coordinator/Control System**: Central server managing job lifecycle
- **Verifier**: Validates configuration files
- **Loader**: Loads and parses YAML configurations
- **Scheduler**: Schedules jobs for execution
- **Queue**: Message queue for decoupling job submission from execution, allowing asynchronous job distribution to runners
- **Database**: Persists job metadata and results
- **Runner**: Executes jobs in Docker containers

## Data Flow

1. **User invokes CLI commands**: User runs commands like `cicd run`, `cicd dry-run`, or `cicd validate`
2. **CLI makes HTTP requests**: CLI translates user commands into HTTP GET/POST requests to the Coordinator server
3. **Coordinator processes requests**:
   - **Verifier** validates the YAML configuration
   - **Loader** parses and loads the configuration
   - **Scheduler** determines job execution order and resource allocation
4. **Metadata persistence**: Coordinator writes job metadata to Database (status: pending, configuration, timestamps, etc.)
5. **Job enqueuing**: Coordinator enqueues jobs to RabbitMQ message queue with execution details (Docker image, script, location, metadata)
6. **Job distribution**: Runner(s) dequeue jobs from the queue when ready to execute
7. **Job execution**: Runner executes the job in an isolated Docker container
8. **Result collection**: Runner returns execution results to Coordinator (status, timestamps, exit code, logs)
9. **Result persistence**: Coordinator updates job status and results in Database
10. **Response to user**: CLI retrieves and displays job results and status to the user