# Changes

## Ping pong (alternating stable with preview)

Added support for the [pingPong](https://rollouts-plugin-trafficrouter-gatewayapi.readthedocs.io/en/latest/features/ping-pong/) traffic routing strategy  on all route types (HTTPRoute, GRPCRoute, TCPRoute, TLSRoute) allowing zero-downtime deployments for long lived connections. This was previously available
only for [ALB](https://argo-rollouts.readthedocs.io/en/stable/features/traffic-management/alb/#zero-downtime-updates-with-ping-pong-feature) and [istio](https://argo-rollouts.readthedocs.io/en/stable/features/traffic-management/istio/#ping-pong).

Ping pong Requires Argo Rollouts v1.10.0 or later.

## Fine grained canaries

Added support for `maxTrafficWeight` for values over 100.

Note that this MAY change the behavior of rollouts with `maxTrafficWeight` already set, where all `setWeight` use values not more than 100. For instance, `setWeight: 10` with `maxTrafficWeight: 1000` will now route **1%** of the traffic, **NOT** 10%. If you use such settings, you should review the `setWeight` values.

## Other changes

- Fixed experiment percentage calculation
- Tested with HaProxy gateway API implementation
- Started e2e tests for experiments
- Added code coverage calculation in CI

