# CI/CD System — High-Level Design

## Architecture Overview

This CI/CD system follows a microservices architecture with clear separation of concerns across seven layers: User Interface, API Gateway, Controller Services, Message Queue, Worker Layer, Data Layer, and Container Runtime.
```mermaid
graph LR

%% =====================
%% User Layer
%% =====================
    subgraph UserLayer["User Layer"]
        CLI["CLI Client<br/>(cicd)"]
        Git["Git Repo<br/>(.pipelines/*.yaml)"]
    end


%% =====================
%% API Layer
%% =====================
    subgraph APILayer["API Layer"]
        Gateway["API Gateway<br/>(8000)"]
    end


%% =====================
%% Controller Layer
%% =====================
    subgraph Controller["Controller Services"]
        ValService["Validation Service<br/>(8001)"]
        ExecService["Execution Service<br/>(8002)"]
        ReportService["Reporting Service<br/>(8004)"]
    end


%% =====================
%% Message Queue Layer
%% =====================
    subgraph QueueLayer["Message Queue"]
        Queue["Job Queue<br/>(Redis/RabbitMQ)"]
    end


%% =====================
%% Worker Layer
%% =====================
    subgraph WorkerLayer["Worker Layer"]
        WorkerService["Worker Service<br/>(8003)"]
    end


%% =====================
%% Runtime
%% =====================
    subgraph Runtime["Container Runtime"]
        Docker["Docker Engine"]
    end


%% =====================
%% Data Layer
%% =====================
    subgraph DataLayer["Data Layer"]
        DB["PostgreSQL"]
    end


%% =====================
%% User Flow (1–3)
%% =====================

    CLI -->|1. Read Config| Git
    CLI -->|2. Request| Gateway

    Gateway -->|3a. POST /validate| ValService
    Gateway -->|3b. POST /run| ExecService
    Gateway -->|3c. GET /report| ReportService


%% =====================
%% Validation (4–5a)
%% =====================

    ValService -->|4. Validate YAML| Gateway
    ExecService -->|5a. Validate Pipeline| ValService
    ValService -.->|Valid / Invalid| ExecService


%% =====================
%% Execution (5b–5d)
%% =====================

    ExecService -->|5b. Create Run| DB
    DB -.->|run_id| ExecService

    ExecService -->|5c. Enqueue Job| Queue
    Queue -->|5d. Consume Job| WorkerService
    WorkerService -->|5e. Publish Status| Queue
    Queue -.->|5f. Status Event| ExecService

    ExecService -->|5g. Update State| DB


%% =====================
%% Worker Runtime (6)
%% =====================

    WorkerService -->|6a. Pull Image| Docker
    WorkerService -->|6b. Start Container| Docker

    Docker -.->|Logs| WorkerService
    Docker -.->|Exit Code| WorkerService

    WorkerService -->|6c. Store Logs| DB
    WorkerService -->|6d. Cleanup| Docker


%% =====================
%% Reporting (7)
%% =====================

    ReportService -->|7. Query| DB
    DB -.->|Result| ReportService

    ReportService -.->|Report Data| Gateway
    Gateway -.->|Response| CLI
```
## Design Summary

The system adopts a **service-oriented architecture** where:

- **CLI client** reads pipeline definitions from Git repositories and communicates with the backend through an API Gateway
- **API Gateway** (port 8000) routes requests to specialized services using REST/gRPC
- **Controller layer** contains three independent services:
   - Validation Service (port 8001): YAML parsing, schema validation, and cycle detection
   - Execution Service (port 8002): Pipeline orchestration, job scheduling, and state management
   - Reporting Service (port 8004): Query aggregation and result formatting
- **Message Queue** (Redis/RabbitMQ) decouples the Execution Service from Worker Service through asynchronous job dispatch and status updates
- **Worker Service** (port 8003) manages job execution through Docker containers and collecting logs
- **PostgreSQL database** (port 5432) persists pipeline runs, stages, jobs, and execution logs
- **Docker Engine** handles container lifecycle and image management

## Pros and Cons

### Pros

1. **Clear Separation of Concerns**
   - Each service has a well-defined responsibility, making the codebase easier to maintain and test
   - Validation logic is decoupled from execution, allowing independent development and updates

2. **Scalability**
   - Microservices architecture allows horizontal scaling of individual components
   - Worker service can be scaled independently to handle varying workload demands
   - Multiple worker instances can process jobs in parallel from the same queue
   - Message queue naturally supports load distribution across multiple workers

3. **Flexibility and Extensibility**
   - Easy to add new services (e.g., notification service, metrics service) without affecting existing ones
   - Can support multiple protocols (REST/gRPC) through the API Gateway
   - Worker service can support different container runtimes with minimal changes

4. **Fault Isolation**
   - Failure in one service (e.g., reporting) doesn't bring down the entire system
   - Worker failures are contained and don't affect the controller services
   - Message queue provides durability: jobs persist even if workers crash

5. **Technology Independence**
   - Each service can be implemented in different languages/frameworks if needed
   - Database and cache layers can be swapped without major architectural changes

6. **Clean API Design**
   - API Gateway provides a single entry point, simplifying client implementation
   - Service-to-service communication is explicit and traceable

7. **Asynchronous Job Processing**
   - Message queue decouples job dispatch from execution, improving system responsiveness
   - Execution Service can enqueue jobs and continue without blocking on worker availability
   - Built-in retry mechanisms and dead-letter queues for failure recovery
   - Workers can process jobs at their own pace, preventing overload

### Cons

1. **Increased Complexity**
   - Multiple services require more deployment and monitoring infrastructure
   - Service discovery, inter-service communication, and network configuration add complexity
   - More moving parts increase the potential for configuration errors
   - Message queue adds another component to deploy, monitor, and maintain

2. **Network Overhead**
   - Inter-service communication introduces latency compared to monolithic design
   - Each hop (CLI → Gateway → Service → Queue → Worker → Database) adds network round-trip time

3. **Eventual Consistency Challenges**
   - Asynchronous processing means job status updates are not immediately reflected
   - Need to handle scenarios where Execution Service queries status before Worker has published updates
   - Requires careful design of status polling or webhook mechanisms for the CLI client

4. **Message Queue Operational Overhead**
   - Requires expertise in queue configuration (prefetch limits, acknowledgment modes, persistence)
   - Need to monitor queue depth, processing rates, and dead-letter queues
   - Message ordering guarantees may require additional complexity depending on queue technology
   - Potential for message duplication if acknowledgments fail (requires idempotent job processing)

5. **Database as Log Sink Limitations**
   - Storing raw execution logs directly in PostgreSQL can cause table bloat over time
   - Large log volumes can degrade query performance for pipeline status checks
   - Competing workload: transactional queries (pipeline status) vs. append-heavy log writes
   - May require eventual migration to dedicated log storage (e.g., S3, Elasticsearch)

## Design Rationale

This architecture was chosen for the following reasons:

1. **Educational Value**: The microservices approach provides hands-on experience with distributed systems concepts, which aligns with the course objectives around container orchestration and service communication.

2. **Industry Relevance**: Modern CI/CD platforms (Jenkins X, GitLab CI, GitHub Actions) use similar distributed architectures with message queues for job dispatch, making this design representative of production systems.

3. **Clear Learning Boundaries**: Each service represents a distinct domain (validation, execution, reporting), making it easier to understand, implement, and demonstrate specific functionality.

4. **Demonstration of Container Orchestration**: The separation between controller and worker layers showcases container management patterns similar to Kubernetes' control plane and worker node architecture.

5. **Incremental Development**: Services can be implemented and tested independently, allowing for iterative development and easier debugging during the project timeline.

6. **Realistic Scaling Scenarios**: The design naturally supports demonstrating horizontal scaling concepts, particularly with the Worker Service handling parallel job execution. The message queue enables multiple workers to process jobs concurrently without coordination overhead.

7. **API Gateway Pattern**: Including an API Gateway demonstrates understanding of modern cloud-native architectures and provides a clean interface for future extensions (webhooks, UI, multiple clients).

8. **Decoupling through Messaging**: The message queue pattern demonstrates key distributed systems concepts including:
   - Asynchronous communication and eventual consistency
   - Load leveling and backpressure management
   - Fault tolerance through message persistence and retry mechanisms
   - Horizontal scalability of worker pools

9. **Pragmatic Trade-offs for Project Scope**:
   - Using a lightweight message queue (Redis Streams or RabbitMQ) balances production-grade patterns with development simplicity
   - Direct PostgreSQL log storage keeps the tech stack manageable, though production systems would typically use dedicated log aggregation solutions (ELK stack, Loki, CloudWatch)
   - These trade-offs prioritize rapid development and clear demonstration of core CI/CD and distributed systems concepts