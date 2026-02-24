# Week 4


# Completed tasks

| Task                                                                                                                                               | Weight | Assignee  | 
|----------------------------------------------------------------------------------------------------------------------------------------------------|--------|-----------| 
| [ [Design] Design database schema for report store (pipeline/stage/job runs)](https://github.com/CS7580-SEA-SP26/e-team/issues/127)                | S      | Eugenia-Z |
| [[Database Service] Implement reportstore package (DB interaction API for report/execution)](https://github.com/CS7580-SEA-SP26/e-team/issues/130) | M      | Eugenia-Z|
| [[Database Service] Concurrency-safe run_no and write operations](https://github.com/CS7580-SEA-SP26/e-team/issues/131)                            | S      | Eugenia-Z|
| [[Infra]Make validation/execution services configurable](https://github.com/CS7580-SEA-SP26/e-team/issues/120)                                     | S      | Asurkatha|
| [[CLI]Run command: call API gateway instead of execution service directly](https://github.com/CS7580-SEA-SP26/e-team/issues/124)                   | S      | Asurkatha|
| [[CLI] Add report subcommand scaffold with flag](https://github.com/CS7580-SEA-SP26/e-team/issues/139)                                             | S      | Asurkatha|
| [[CLI] Validate report flag combinations](https://github.com/CS7580-SEA-SP26/e-team/issues/140)                                                    | S      | Asurkatha|
| [[CLI] Add gateway client method for /report](https://github.com/CS7580-SEA-SP26/e-team/issues/141)                                                | S      | Asurkatha|
| [[CLI] Add report formatters in format.go](https://github.com/CS7580-SEA-SP26/e-team/issues/142)                                                   | S      | Asurkatha|
| [[Gateway] Add /report route and handler](https://github.com/CS7580-SEA-SP26/e-team/issues/144)                                                    | M      | Asurkatha|
| [[Gateway]Add reporting proxy client](https://github.com/CS7580-SEA-SP26/e-team/issues/145)                                                        | M      | Asurkatha|
| [[Reporting Service] Implement report orchestration using store read APIs](https://github.com/CS7580-SEA-SP26/e-team/issues/146)                   | M      | Asurkatha|
| [[Infra/Dev] Add reporting service to local start script](https://github.com/CS7580-SEA-SP26/e-team/issues/147)                                    | S      | Asurkatha|
| [[Execution Service] Add data persistence to Execution Service for pipeline run tracking](https://github.com/CS7580-SEA-SP26/e-team/issues/148)    |M|xueyulinn|
| [[Execution service] – integrate Git for workspace extraction](https://github.com/CS7580-SEA-SP26/e-team/issues/121)                               |M|xueyulinn|


# Carry over tasks

Please note: the carry-over items in the backlog are intended to improve codebase design and quality; they do not indicate that existing functionality is incomplete.

| Task | Weight | Assignee |
| ---- | ------ | -------- |
| [Consolidate HTTP request/response types in types.go](https://github.com/CS7580-SEA-SP26/e-team/issues/122) | S | Eugenia-Z | 
| [Streamline error reporting format across services and CLI](https://github.com/CS7580-SEA-SP26/e-team/issues/123) | S | Eugenia-Z |




# What worked this week?
1. Early alignment on data flow ensured a clear system architecture — all existing services interact with the database exclusively through data service APIs, rather than accessing the database directly.
2. Easy division of labor based on expertise and task complexity.

# What did not work this week?
1. Extremely minor merge conflicts due to simultaneous changes in the same file in different PRs.

# Design updates

1. We have added database schema into the design doc. Please check it out [here](https://github.com/CS7580-SEA-SP26/e-team/blob/review/dev-docs/design/design-db-schema.md).
2. Updated high level architecture diagram to include the data layer. [here](https://github.com/CS7580-SEA-SP26/e-team/blob/review/dev-docs/design/high-level-design.md)


> | Task | Points|
> | --- | --- | 
> | Issue are linked in the weekly report and point to the right issue on GitHub | 2 | 
> | Issues marked as DONE in the report are closed in GitHub | 2 | 
> | Issues marked as INCOMPLETE in the report are not closed in GitHub | 2 | 
> | Linked Issues have at least 1 linked PR | 4 | 
> | Linked Issues on GitHub have a clear title and description | 4 | 
> | Linked Issues on GitHub have 1 assignee | 2 | 
> | Linked Issues on GitHub have estimates | 2 | 
> | **TOTAL**  | **18** |
 