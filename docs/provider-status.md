# Provider Status

Several Service Mesh and Gateway solutions are already implementing
the Gateway API. You can find a contributed list of known implementations at the [Gateway API website](https://gateway-api.sigs.k8s.io/implementations/).

All providers should work out of the box with Argo Rollouts and the Gateway plugin.

For convenience we are including here a list of those actually tested with the plugin along with the related example (if applicable).


| Provider   |    Works | Code |
|------------|----------|---------|
| [Cilium](https://cilium.io/)     | yes      | [Example](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/tree/main/examples/cilium)    |
| [Envoy Gateway](https://gateway.envoyproxy.io/)     | yes      | [Example](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/tree/main/examples/envoygateway)    |
| [Gloo Gateway](https://docs.solo.io/gloo-gateway/v2/)     | yes      | [Example](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/tree/main/examples/gloo-gateway)    |
| [Google Cloud](https://cloud.google.com/kubernetes-engine/docs/concepts/gateway-api)     | yes      | [Example](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/tree/main/examples/google-cloud)    |
| [Kong](https://docs.konghq.com/kubernetes-ingress-controller/latest/concepts/gateway-api/)     | yes      | [Example](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/tree/main/examples/kong)    |
| [NGINX Gateway](https://github.com/nginxinc/nginx-gateway-fabric)     | yes      | [Example](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/tree/main/examples/nginx)    |
| [Traefik](https://doc.traefik.io/traefik/providers/kubernetes-gateway/)     | yes      | [Example](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/tree/main/examples/traefik)    |

Note that these examples are included just for completeness. You should be able
to use any solution that implements the Gateway API. 

!!! note
    We are always looking for more tested implementations. If you have tried the plugin with a provider not listed above please [open a Pull Request](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/pulls) to add it to the list.
