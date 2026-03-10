# Test: `single-grpcroute-filters`

Verifies that existing GRPCRoute filters (RequestHeaderModifier, ResponseHeaderModifier, RequestMirror) are preserved on both the original rule and the newly injected header-match rule when header-based routing is active.


## Steps

| # | Name | Bindings | Try | Catch | Finally | Cleanup |
|:-:|---|:-:|:-:|:-:|:-:|:-:|
| 1 | [create-resources](#step-create-resources) | 0 | 2 | 0 | 0 | 0 |
| 2 | [assert-initial-state](#step-assert-initial-state) | 0 | 1 | 0 | 0 | 0 |
| 3 | [wait-for-initial-healthy](#step-wait-for-initial-healthy) | 0 | 1 | 0 | 0 | 0 |
| 4 | [trigger-canary](#step-trigger-canary) | 0 | 1 | 0 | 0 | 0 |
| 5 | [assert-filters-preserved-in-header-route](#step-assert-filters-preserved-in-header-route) | 0 | 1 | 0 | 0 | 0 |
| 6 | [assert-rollout-healthy](#step-assert-rollout-healthy) | 0 | 1 | 0 | 0 | 0 |
| 7 | [assert-cleanup](#step-assert-cleanup) | 0 | 1 | 0 | 0 | 0 |

### Step: `create-resources`

Create the GRPCRoute with 3 filters and the Rollout. Uses pause:{duration:30s} for auto-promotion.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `apply` | 0 | 0 | *No description* |
| 2 | `apply` | 0 | 0 | *No description* |

### Step: `assert-initial-state`

Plugin initializes the GRPCRoute with canary weight 0; the 3 original filters are present.

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

### Step: `assert-filters-preserved-in-header-route`

Both the original rule and the injected header-match rule must carry all 3 filters.

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

Plugin removes the header-match rule; original rule still has its 3 filters with canary weight 0.

#### Try

| # | Operation | Bindings | Outputs | Description |
|:-:|---|:-:|:-:|---|
| 1 | `assert` | 0 | 0 | *No description* |

---

