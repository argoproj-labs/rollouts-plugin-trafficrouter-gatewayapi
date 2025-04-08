# Argo Rollouts Gateway API Experiment Support

This feature adds support for conducting experiments with Argo Rollouts using the Kubernetes Gateway API. Experiments allow you to test multiple versions of your application simultaneously with precise control over traffic distribution.

## Overview

When using the Gateway API traffic router with Argo Rollouts, you can now define experiments that:

- Automatically adjust traffic weights in HTTPRoutes for the additional services created for experiment variants
- Clean up experiment services when experiments complete

## How It Works

The plugin automatically:

1. Detects when an experiment is active in a rollout
2. Adjusts the stable service weight to accommodate experiment traffic
3. Adds experiment services to the HTTPRoute with appropriate weights
4. Removes experiment services when the experiment completes

## Example Usage

The included example demonstrates a rollout with an experiment step that tests:
- A baseline variant based on the stable version (10% traffic)
- A canary variant based on the new version (10% traffic)

During the experiment:
- The stable service receives 80% of traffic (reduced from 100%)
- The canary service continues to receive 0% traffic
- The experiment variants receive their specified weights ( 10% , 10%)

After the experiment completes, traffic distribution returns to normal with stable receiving 100% until the next step begins.

### Sample Manifest

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: demo-app
  namespace: demo
spec:
  strategy:
    canary:
      canaryService: demo-app-canary
      stableService: demo-app-stable
      trafficRouting:
        plugins:
          argoproj-labs/gatewayAPI:
            httpRoute: demo-app-route
            namespace: demo
      steps:
      - experiment:
          duration: 5m
          templates:
          - name: experiment-baseline
            specRef: stable
            service:
              name: demo-app-exp-baseline
            weight: 10
          - name: experiment-canary
            specRef: canary
            service:
              name: demo-app-exp-canary
            weight: 10
      # Remaining steps...
```

## Implementation Details

The experiment handler:

1. Identifies the matching rule in the HTTPRoute for the rollout
2. Checks if an experiment is active by examining `rollout.Status.Canary.CurrentExperiment`
3. For active experiments:
   - Sets the stable service weight to 80%
   - Adds experiment services from `rollout.Status.Canary.Weights.Additional`
4. For inactive experiments:
   - Removes any experiment services from the HTTPRoute
   - Resets the stable service weight to 100%

## Requirements

- Kubernetes cluster with Gateway API CRDs installed
- Argo Rollouts v1.5.0 or newer
- Simple HTTP Gateway (TLS configuration optional)

## See Also

- [Argo Rollouts Documentation](https://argoproj.github.io/argo-rollouts/)
- [Gateway API Documentation](https://gateway-api.sigs.k8s.io/)
- [Experiment Documentation](https://argoproj.github.io/argo-rollouts/features/experiment/)