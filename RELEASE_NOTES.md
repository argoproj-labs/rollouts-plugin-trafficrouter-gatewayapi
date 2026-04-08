## Refactored version of the plugin

**Possible breaking change. Please read all notes**

This release contains a refactoring of the plugin internal where the configmap is no longer needed for header based routing

TL;DR

You can be in one of 3 groups

1. If you didn't use header based routing, everything is ok and you should be able to upgrade without any issues
1. If you came from Istio and want to use header based routing with Gateway API this release will make your life easier
1. If you don't care about Istio and you already used header based routing with another traffic provider this release _might_ break your existing processes
1. In all cases you need Gateway API 1.2+ in your cluster

## All changes are in the header based routing code

There are many code changes with this release but all of them are scoped in header based routing. If you don't use header based routing for either GRPC or HTTP
this release is identical to v0.12.0

## The plugin now follows the istio based implementation of Argo Rollouts core

The plugin now should have the same functionality for header based routing as it was with [Istio in Argo Rollout core](https://argo-rollouts.readthedocs.io/en/stable/features/traffic-management/istio/).

Mainly

- Header based routes as stored as separate rules/matches in the original http routes using name (same as istio)
- setWeight directives do not affect header based rules ([Bug 158](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/issues/158) should be fixed). Previously, weight updates would also modify weights inside header-based rules, potentially overriding them.
- The configmap that was used to track header based routes was problematic [for several issues](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/issues/95) and is now completely removed. 
- You will see a new name property in all your managed rules. This is used by the plugin.

In general if you are coming from Istio, the Gateway API plugin should offer the same semantics.

This [integration test](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/blob/main/test/e2e/chainsaw/advanced-header-based-httproute/resources/rollout.yaml) better shows the new behavior. There is a [similar test for GRPC](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/blob/main/test/e2e/chainsaw/advanced-header-based-grpcroute/resources/rollout.yaml).


## Possible breakage for existing header based rollouts

As we all know, one person's fix is [another person's issue](https://xkcd.com/1172/). If you used header based routing and for some reason
you actually wanted your header based routes to be affected by setWeight this will not work. Also if you tampered with the configmap for some reason, 
it is completely gone.

You don't need [cleanup scripts anymore](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/issues/95#issuecomment-2728591308).

## Upgrading

If you have existing managed routes it is best to upgrade to this new version while a deployment is NOT in progress. This way the plugin will correctly 
recreate all rules/matches with name.

For people that do not have this choice, there is backwards compatible code that will still use introspection to find header based routes that
were managed by the previous plugin version that still used the configmap. However the code is not bulletproof and before upgrading production it is
better to simply employ a sync window to stop all deployments. The fallback code will be removed in future releases. Note that the fallback
code will **NOT** add the name property to existing managed routes. It will just work to detect them correctly and not break the plugin. Existing rules will receive a name on the next canary cycle after upgrading. So if you have deployments that take days ([instead of the recommended hours](https://argo-rollouts.readthedocs.io/en/latest/best-practices/#understand-your-use-case)) beware that those rules will remain unnamed until the current canary completes.

Once you are certain that things work you can delete the configmap and optionally revoke the RBAC configuration for reading/writing configmaps.


## More technical information

If you are still wondering why did we have to do this major refactoring and why now here is the complete history

The plugin was written in the very early days of the gateway api where http routes were very simple.  Header based routing was added as an extra
feature to the plugin and it turns out that this was super popular with users. 

The main problem with header based routing is that the plugin needs a way to manage all these special rules/matches in the main route.
Initially a configmap was added that was the main storage.

However keeping a configmap and the actual route in sync was problematic. The plugin had a custom mutex and a custom transaction implementation to handle
this.

Hopefully with [Gateway api 1.2 the name property](https://gateway-api.sigs.k8s.io/api-types/httproute/#name-optional) was introduced. So we don't need this configmap anymore as we can simply search rules by name.
By the way this is the same way that Istio does header based routing the main Argo Rollouts controller (it can name destination rules)

This also means that Gateway API v1.2 is a requirement for the plugin. Note that your traffic provider doesn't have to do anything special for named
rules. It just needs to leave them alone and accept the name as an existing parameter.




