# Test: `single-header-based-httproute`

Verifies header-based routing with a single HTTPRoute. The plugin must inject a header-match rule (X-Test: test) during a canary rollout and remove it once the rollout completes.


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [create-resources](#step-create-resources) | 0 | 2 | 0 | 0 | 0 |
| 2 | [assert-initial-state](#step-assert-initial-state) | 0 | 1 | 0 | 0 | 0 |
| 3 | [wait-for-initial-healthy](#step-wait-for-initial-healthy) | 0 | 1 | 0 | 0 | 0 |
| 4 | [trigger-canary](#step-trigger-canary) | 0 | 1 | 0 | 0 | 0 |
| 5 | [assert-header-routing-active](#step-assert-header-routing-active) | 0 | 1 | 0 | 0 | 0 |
| 6 | [assert-rollout-healthy](#step-assert-rollout-healthy) | 0 | 1 | 0 | 0 | 0 |
| 7 | [assert-cleanup](#step-assert-cleanup) | 0 | 1 | 0 | 0 | 0 |

### Step: `create-resources`

Create the HTTPRoute and Rollout. The rollout uses pause:{duration:30s} so it self-promotes.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `apply` | 0 | 0 | *No description* |

### Step: `assert-initial-state`

Plugin initializes the HTTPRoute with 1 rule and canary weight 0.

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

Update the rollout image to start a canary (setWeight:30 -> setHeaderRoute -> pause:30s).

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `patch` | 0 | 0 | *No description* |

### Step: `assert-header-routing-active`

Plugin must have added a header-match rule (X-Test:test) alongside the original weight-split rule.

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

Plugin must remove the header-match rule and reset canary weight to 0 after rollout completes.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | *No description* |

---

