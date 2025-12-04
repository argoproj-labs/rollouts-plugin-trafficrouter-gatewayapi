* Added support for [TLSRoute](https://rollouts-plugin-trafficrouter-gatewayapi.readthedocs.io/en/latest/features/tls/).
* You can now use  [filters with Header based routing](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/issues/87).
* Gateway API routes are labeled while a canary is running to avoid GitOps drift and the label is removed once traffic returns to 100% stable.
