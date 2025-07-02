# Provider Status

Several Service Mesh and Gateway solutions are already implementing
the Gateway API. You can find a contributed list of known implementations at the [Gateway API website](https://gateway-api.sigs.k8s.io/implementations/).

All providers should work out of the box with Argo Rollouts and the Gateway plugin.

!!! warning
    Notice that with 0.x implementations of the Gateway API, only the 0.2.0 release of the 
    plugin will work. Versions from 0.3.0 and upwards need a v1.0+ implementation 
    to work. Plugin version 0.2.0 uses `v1beta1` resources while 0.3.0 needs `gateway.networking.k8s.io/v1` resources

For convenience we are including here a list of those actually tested with the plugin along with the related example (if applicable).


| Provider   |    Version | API Version | Plugin   | Code     |
|------------|------------|-------------| ---------| ---------|
| [Cilium](https://cilium.io/)     |  Unknown      | 0.7.0 | 0.2.0 | [Example](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/tree/main/examples/cilium)    |
| [Envoy Gateway](https://gateway.envoyproxy.io/)     | 0.5.0  |   Unknown  | 0.2.0 | [Example](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/tree/main/examples/envoygateway)    |
| [Gloo Gateway](https://docs.solo.io/gloo-gateway/v2/)     | 2.0.0-beta | 1.0      | 0.2.0 | [Example](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/tree/main/examples/gloo-gateway)    |
| [Google Cloud](https://cloud.google.com/kubernetes-engine/docs/concepts/gateway-api)     | N/A | 0.7.0      | 0.2.0 | [Example](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/tree/main/examples/google-cloud)    |
| [Kong](https://docs.konghq.com/kubernetes-ingress-controller/latest/concepts/gateway-api/)     | 2.9.x  | 0.7.1    | 0.2.0 | [Example](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/tree/main/examples/kong)    |
| [NGINX Gateway](https://github.com/nginxinc/nginx-gateway-fabric)     | Unknown | 0.8.0      | 0.2.0 | [Example](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/tree/main/examples/nginx)    |
| [Traefik](https://doc.traefik.io/traefik/providers/kubernetes-gateway/)     | 3.1.3 | 1.1      | 0.4.0 | [Example](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/tree/main/examples/traefik)    |
| [Linkerd](https://linkerd.io/)     | 2.13.0 |   Unknown    | 0.2.0 | [Example](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/tree/main/examples/linkerd)    |
| [AWS Gateway API Controller for Amazon VPC Lattice](https://www.gateway-api-controller.eks.aws.dev/latest//)     | 1.1.2 |   1.3.0    | 0.6.0 | [Example](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/tree/main/examples/aws-gateway-api-controller-lattice )    |

Note that these examples are included just for completeness. You should be able
to use any solution that implements the Gateway API. 

!!! note
    We are always looking for more tested implementations. If you have tried the plugin with a provider not listed above please [open a Pull Request](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/pulls) to add it to the list.
