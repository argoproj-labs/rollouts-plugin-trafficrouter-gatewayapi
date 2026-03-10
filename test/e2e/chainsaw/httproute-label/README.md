# Test: `httproute-label`

Verifies the in-progress label lifecycle on an HTTPRoute. The plugin adds 'rollouts.argoproj.io/gatewayapi-canary=in-progress' when a canary is active and removes it once the rollout completes.


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [create-resources](#step-create-resources) | 0 | 2 | 0 | 0 | 0 |
| 2 | [assert-label-absent-initially](#step-assert-label-absent-initially) | 0 | 1 | 0 | 0 | 0 |
| 3 | [wait-for-initial-healthy](#step-wait-for-initial-healthy) | 0 | 1 | 0 | 0 | 0 |
| 4 | [trigger-canary](#step-trigger-canary) | 0 | 1 | 0 | 0 | 0 |
| 5 | [assert-label-present-during-canary](#step-assert-label-present-during-canary) | 0 | 1 | 0 | 0 | 0 |
| 6 | [assert-rollout-healthy](#step-assert-rollout-healthy) | 0 | 1 | 0 | 0 | 0 |
| 7 | [assert-label-removed-after-completion](#step-assert-label-removed-after-completion) | 0 | 1 | 0 | 0 | 0 |

### Step: `create-resources`

Create the HTTPRoute and Rollout. The rollout uses pause:{duration:30s} so it self-promotes.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `apply` | 0 | 0 | *No description* |

### Step: `assert-label-absent-initially`

Plugin must not apply the in-progress label when canary weight is 0.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | *No description* |

### Step: `wait-for-initial-healthy`

Wait for the initial rollout to reach Healthy before triggering a canary.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | *No description* |

### Step: `trigger-canary`

Update the rollout image to start a canary (setWeight:30 -> pause:30s).

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `patch` | 0 | 0 | *No description* |

### Step: `assert-label-present-during-canary`

Plugin must apply the in-progress label once canary traffic is active.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | *No description* |

### Step: `assert-rollout-healthy`

Wait for the canary to complete (30s pause auto-expires).

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | *No description* |

### Step: `assert-label-removed-after-completion`

Plugin must remove the in-progress label once the rollout completes.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | *No description* |

---

