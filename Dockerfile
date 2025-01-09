FROM --platform=$BUILDPLATFORM golang:1.22.5 AS builder

ENV GO111MODULE=on
ARG TARGETARCH

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags "-s -w" -o rollouts-plugin-trafficrouter-gatewayapi .

FROM alpine:3.19.0

ARG TARGETARCH

USER 999

COPY --from=builder /app/rollouts-plugin-trafficrouter-gatewayapi /bin/
