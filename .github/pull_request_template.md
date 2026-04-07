<!--
Note on DCO:

If the DCO action in the integration test fails, one or more of your commits are not signed off. Please click on the *Details* link next to the DCO action for instructions on how to resolve this.
-->

## Description of this PR

Explain what this PR does, what is the use case and if it breaks existing behavior for other users

## Checklist:

* [ ] I have added a brief description of why this PR is necessary and/or what this PR solves.
* [ ] I have signed off all my commits as required by [DCO](https://github.com/argoproj/argoproj/blob/master/community/CONTRIBUTING.md#legal)
* [ ] My build is green (Existing flaky test can fail).
* [ ] I have written unit and/or e2e tests for my change. PRs without these are unlikely to be merged. 
* [ ] The tests actually define testcases about the feature/fix implemented
* [ ] I have run all tests locally and they pass 
* [ ] I haven't changed the existing tests in a significant way, as this will break functionality for existing users <sup>1</sup>
* [ ] I've updated documentation as required by this PR.
* [ ] I have used LLM/AI/Agent tools for this PR but I am responsible for all code of this PR
* [ ] I understand what the code does and WHY it works that way according to my use case
* [ ] I understand what the code does and HOW it works in several scenarios
* [ ] I know if my code is just adding new functionality or changing old functionality for existing users
* [ ] My new code is using existing utility functions instead of re-implementing everything again
* [ ] Optional. My organization is added to [USERS.md](https://github.com/argoproj/argo-rollouts/blob/master/USERS.md)

Notes
1. Unless a discussion has happened already in an issue and the breaking change is deemed acceptable
