# Week 1


# Completed tasks

| Task                                                                                                                    | Weight | Assignee  | 
|-------------------------------------------------------------------------------------------------------------------------|--------|-----------| 
| [ [[Docs] Create initial high-level architecture design (CLI / REST Service / Data Store)] ]( https://github.com/CS7580-SEA-SP26/e-team/issues/8)                         | Small  | Eugenia-Z | 
| [[Docs] Define project tech stack and CI/CD strategy            ](https://github.com/CS7580-SEA-SP26/e-team/issues/10)  | Small  | Eugenia-Z | 
| [[CLI] Implement verify subcommand to validate pipeline YAML](https://github.com/CS7580-SEA-SP26/e-team/issues/13)      | Medium | Eugenia-Z|
| [[Data Model] Add Pipeline data structure ](https://github.com/CS7580-SEA-SP26/e-team/issues/33)                        | Medium | Eugenia-Z|
| [[Validator] Validate pipeline structure and references](https://github.com/CS7580-SEA-SP26/e-team/issues/21)           | Medium | Eugenia-Z|
| [[Validator] Detect dependency cycles in needs (DAG validation)](https://github.com/CS7580-SEA-SP26/e-team/issues/19)   | Medium | Eugenia-Z|
| [[Validator] Validate needs references](https://github.com/CS7580-SEA-SP26/e-team/issues/18)                            | Medium | Eugenia-Z|
| [[Error Reporting] Detect and report incorrect types for YAML keys.](https://github.com/CS7580-SEA-SP26/e-team/issues/9) | Small  |Eugenia-Z|
| [[Error Reporting] Report Cycles in YAML](https://github.com/CS7580-SEA-SP26/e-team/issues/7)                           | Small  |Eugenia-Z|
| [[Error Reporting] Standardize error format](https://github.com/CS7580-SEA-SP26/e-team/issues/6)                        | Small  | Eugenia-Z|
| [[Test] Unit tests for validator rules using fixtures](https://github.com/CS7580-SEA-SP26/e-team/issues/22)             | Small  |Eugenia-Z|
| [[Validator] Validate unique job names](https://github.com/CS7580-SEA-SP26/e-team/issues/17)                            | Small  |Eugenia-Z|
| [[CLI] Enforce execution only at Git repository root](https://github.com/CS7580-SEA-SP26/e-team/issues/11)              | Small  | xueyulinn
| [[CLI] Enforce .pipelines directory at repository root](https://github.com/CS7580-SEA-SP26/e-team/issues/12)            | Small  | xueyulinn
| [[CLI] Support default pipeline file path for verify](https://github.com/CS7580-SEA-SP26/e-team/issues/14)              | Small  | xueyulinn
| [[CLI] Validate file path is relative to repository root](https://github.com/CS7580-SEA-SP26/e-team/issues/15)          | Small  | xueyulinn
| [[CLI] Add help and usage text for verify subcommand](https://github.com/CS7580-SEA-SP26/e-team/issues/16)              | Small  | xueyulinn
| [[Github Config] Create Pipelines for this repo](https://github.com/CS7580-SEA-SP26/e-team/issues/23)                   | Small  | xueyulinn
| [[Github Config] Fix bugs for repo pipelines](https://github.com/CS7580-SEA-SP26/e-team/issues/27)                      | Small  | xueyulinn
| [[Test] Add pipeline YAML fixtures for validation scenarios](https://github.com/CS7580-SEA-SP26/e-team/issues/20)       | Small  | Asurkatha
| [[CLI] Accept folder PATH as input](https://github.com/CS7580-SEA-SP26/e-team/issues/38)                                | Small  | Asurkatha
| [[Testing] Balance tests to match the adjsuted format](https://github.com/CS7580-SEA-SP26/e-team/issues/46)             | Medium | Asurkatha
| [[CLI] Adjust "cicd" as the head command](https://github.com/CS7580-SEA-SP26/e-team/issues/39)                          | Small  | Asurkatha
| [[Refactor] Refactor code (parser.go, check_format.go)](https://github.com/CS7580-SEA-SP26/e-team/issues/42)            | Medium | Asurkatha
| [[Error Reporting] Report error on needs with jobs from different jobs](https://github.com/CS7580-SEA-SP26/e-team/issues/43) | Small  | Asurkatha
# Carry over tasks

> List all issues that were planned for this week but did not get DONE
> Include
> 1. A link to the Issue
> 2. The total weight (points or T-shirt size) allocated to the issue
> 3. The team member assigned to the task. This has to be 1 person!

| Task | Weight | Assignee |
| ---- | ------ | -------- |
|  -   |    -   |    -     |




# What worked this week?

1. Proactive discussion of design decisions and tradeoffs early in the sprint helped the team align on architecture designs before implementation began
2. Extensive testing and bug-fixes through collective team efforts ensured validation logic was robust and edge cases were properly handled
3. Regular team meetings and communication helped maintain alignment

# What did not work this week?

1. PRs were merged too quickly without detailed review, creating refactoring debt for future work. We should establish a minimum review time or require multiple approvers for architectural changes.
2. Unclear repository structure discussion: Initial structure decisions were not fully discussed, leading to multiple refactorings and workflow updates. This caused confusion among team members and CI/CD failures

# Design updates

> If changes have been made to the overall design approach for the project, least the updates here. Link to documents (or updates to documents) that describe in detail what these changes are.



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
 
