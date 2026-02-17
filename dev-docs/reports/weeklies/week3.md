# Week 2

# Completed tasks

| Task                                                                                                                                | Weight | Assignee  |
| ----------------------------------------------------------------------------------------------------------------------------------- |------| --------- |
| [[CLI] Run cmd inialization](https://github.com/CS7580-SEA-SP26/e-team/issues/109)                                                  | M    | xueyulinn |
| [[CLI] Implement cicd run to send pipeline execution request via API Gateway](https://github.com/CS7580-SEA-SP26/e-team/issues/111) | S    | xueyulinn |
| [[Execution Service]Create execution service](https://github.com/CS7580-SEA-SP26/e-team/issues/112)                                 | M    | xueyulinn |
| [[Execution Service]Start execution service as an HTTP server](https://github.com/CS7580-SEA-SP26/e-team/issues/113)                | S    | xueyulinn |
| [[Worker Layer] Worker Service Project Setup ](https://github.com/CS7580-SEA-SP26/e-team/issues/86)|    S | Eugenia-Z |
|[[Worker Layer] Docker Container Manager Implementation](https://github.com/CS7580-SEA-SP26/e-team/issues/87) | L    | Eugenia-Z |
|[[Worker Layer] Job Processor Core Logic](https://github.com/CS7580-SEA-SP26/e-team/issues/88)| M    | Eugenia-Z |
|[[Worker Layer] Implement Worker HTTP Service](https://github.com/CS7580-SEA-SP26/e-team/issues/89) | M    | Eugenia-Z |
|[[Worker Layer] End-to-End Integration Test ](https://github.com/CS7580-SEA-SP26/e-team/issues/90) | M  | Eugenia-Z |
|[[Worker Layer] Documentation & Optimization](https://github.com/CS7580-SEA-SP26/e-team/issues/91) | S    | Eugenia-Z |
| [[Project Structure Refactoring] Refactor: Adopt Microservice Architecture](https://github.com/CS7580-SEA-SP26/e-team/issues/92) | M | Eugenia-Z |
| [[Refactor] Extract Planner and Formatter from Dryrun](https://github.com/CS7580-SEA-SP26/e-team/issues/93) | M | Eugenia-Z |
| [[Bug Fix] Worker runs script via shell so pipeline commands work](https://github.com/CS7580-SEA-SP26/e-team/issues/116) | S | Eugenia-Z |
| [[Worker Layer] Mount workspace and fix timeouts for end-to-end pipeline run](https://github.com/CS7580-SEA-SP26/e-team/issues/117)| S | Eugenia-Z |
# Carry over tasks

Please note: the carry-over items in the backlog are intended to improve codebase design and quality; they do not indicate that existing functionality is incomplete.


| Task | Weight | Assignee |
| -- |--------| ---- |
| [Make validation/execution services configurable](https://github.com/CS7580-SEA-SP26/e-team/issues/120)   | S      | Asurkatha     |
| [Execution service – integrate Git for workspace extraction](https://github.com/CS7580-SEA-SP26/e-team/issues/121) | M |xueyulinn|
| [Consolidate HTTP request/response types in types.go](https://github.com/CS7580-SEA-SP26/e-team/issues/122) | S | Eugenia-Z | 
| [Streamline error reporting format across services and CLI](https://github.com/CS7580-SEA-SP26/e-team/issues/123) | S | Eugenia-Z |
| [Run command: call API gateway instead of execution service directly ](https://github.com/CS7580-SEA-SP26/e-team/issues/124) | S |Asurkatha|



# What worked this week?

1. Clear division of labor on Day 1 helped each team member better estimate their workload.
2. We received quick feedback while integrating service communications, which helped us resolve issues efficiently. 
3. Early alignment on the major refactoring of the codebase kept individual dev work smooth and consistent with the agreed design.
4. Each standalone microservice was delivered to a high quality, so the integration tests ran smoothly.

# What did not work this week?
1. We largely followed an approach of defining API contracts incrementally and refactoring later, rather than defining a clear API contract upfront. Although this sped up individual development, it required extra cleanup for maintainability.

# Design updates
Design remains consistent with last sprint. 

> | Task                                                                         | Points |
> | ---------------------------------------------------------------------------- | ------ |
> | Issue are linked in the weekly report and point to the right issue on GitHub | 2      |
> | Issues marked as DONE in the report are closed in GitHub                     | 2      |
> | Issues marked as INCOMPLETE in the report are not closed in GitHub           | 2      |
> | Linked Issues have at least 1 linked PR                                      | 4      |
> | Linked Issues on GitHub have a clear title and description                   | 4      |
> | Linked Issues on GitHub have 1 assignee                                      | 2      |
> | Linked Issues on GitHub have estimates                                       | 2      |
> | **TOTAL**                                                                    | **18** |
