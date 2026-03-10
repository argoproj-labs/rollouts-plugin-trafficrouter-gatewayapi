# Test: `single-grpcroute`

Verifies basic weight-based canary traffic splitting with a single GRPCRoute. The plugin must set canary weight to 30 when a canary is active and reset it to 0 once the rollout completes.


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [create-resources](#step-create-resources) | 0 | 2 | 0 | 0 | 0 |
| 2 | [assert-initial-state](#step-assert-initial-state) | 0 | 1 | 0 | 0 | 0 |
| 3 | [wait-for-initial-healthy](#step-wait-for-initial-healthy) | 0 | 1 | 0 | 0 | 0 |
| 4 | [trigger-canary](#step-trigger-canary) | 0 | 1 | 0 | 0 | 0 |
| 5 | [assert-canary-weight-active](#step-assert-canary-weight-active) | 0 | 1 | 0 | 0 | 0 |
| 6 | [assert-rollout-healthy](#step-assert-rollout-healthy) | 0 | 1 | 0 | 0 | 0 |
| 7 | [assert-cleanup](#step-assert-cleanup) | 0 | 1 | 0 | 0 | 0 |

### Step: `create-resources`

Create the GRPCRoute and Rollout. The rollout uses pause:{duration:30s} so it self-promotes.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `apply` | 0 | 0 | *No description* |

### Step: `assert-initial-state`

Plugin initializes the GRPCRoute with canary weight 0.

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

### Step: `assert-canary-weight-active`

Plugin must have set canary weight to 30 and stable to 70.

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

### Step: `assert-cleanup`

Plugin must reset canary weight to 0 after rollout completes.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | *No description* |

---

