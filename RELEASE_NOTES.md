# Changes

## Prefix header matches are now escaped and anchored

A `setHeaderRoute` step with a `prefix` header match is turned into a Gateway API `RegularExpression` header match, because Gateway API has no prefix match type for headers. The plugin used to build that expression as `<prefix>.*`, with the prefix inserted verbatim. It now builds `^<prefix>.*`, with the prefix escaped.

Two things change for existing users:

- Regular expression characters in the prefix are no longer interpreted. `prefix: v1.0` also matched `v1x0`, and `prefix: a+b` did not match the value `a+b` at all. Both now behave as a plain prefix.
- The expression is anchored. Gateway API leaves `RegularExpression` semantics to the implementation. Envoy based implementations match the whole header value, so `canary.*` already behaved as a prefix there. Traefik evaluates the expression as an unanchored search, so `canary.*` also matched `not-canary` and sent those requests to the canary.

On Envoy based implementations this matches exactly the same header values as before, as long as the prefix contains no regular expression characters. On Traefik, header values that merely contain the prefix are no longer routed to the canary, which is the prefix behavior documented by Argo Rollouts. If you relied on the previous unanchored matching, use a `regex` header match instead of a `prefix` one.
