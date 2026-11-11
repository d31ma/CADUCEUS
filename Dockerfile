# syntax=docker/dockerfile:1.7

# ── Keycloak (identity provider) ─────────────────────────────────────────────
# Owned by deploy/keycloak. Build the identity-only image with
#   docker build --target mesail-keycloak .
# The all-in-one Mesail image starts FROM mesail-keycloak and sets
# MESAIL_APP_COMMAND; see deploy/keycloak/README.md.
ARG KEYCLOAK_DB=sqlite
ARG UBUNTU_IMAGE=ubuntu:24.04@sha256:fe81560990fae86dc4743202b2cb4d3df98d0bcf949173cecef15ce1127ef852
ARG GOLANG_IMAGE=golang:1.27.3-bookworm@sha256:dc0963f32053e4bc451d24d87e3892d42d01021b61f093e98eb95c0352bb84b0
ARG LAMBDA_ADAPTER_IMAGE=public.ecr.aws/awsguru/aws-lambda-adapter:0.9.1@sha256:46d6625e68cbbdd2efab4a20245977664513f13ffef47915b000d431adcea0b4

FROM eclipse-temurin:21-jdk-noble@sha256:114cabb73033ca4ce7ceec3deb62396bc1b716282bae051ed3c1d82e38fcab2d AS keycloak-providers
COPY --from=quay.io/keycloak/keycloak:26.8.2@sha256:723b1d6e254c0fc96d5cffec860c7e0744d6a3277822b865bb538cf891575e41 /opt/keycloak/lib/lib /keycloak-lib
COPY deploy/keycloak/providers /src
ADD --checksum=sha256:f8dce420a6016278497f4f0560f65602b9db858de141b541cf91a4644ebe4639 \
  https://repo1.maven.org/maven2/org/xerial/sqlite-jdbc/3.54.1.0/sqlite-jdbc-3.54.1.0.jar /providers/sqlite-jdbc.jar
RUN set -e; for provider in sqlite mesail; do \
      javac --release 21 -Xlint:all -Werror -cp '/keycloak-lib/main/*:/keycloak-lib/boot/*' -d "/classes/$provider" \
        $(find "/src/$provider/src/main/java" -name '*.java') \
      && cp -R "/src/$provider/src/main/resources/." "/classes/$provider/" \
      && jar --create --file "/providers/mesail-keycloak-$provider.jar" -C "/classes/$provider" . ; \
    done

FROM quay.io/keycloak/keycloak:26.8.2@sha256:723b1d6e254c0fc96d5cffec860c7e0744d6a3277822b865bb538cf891575e41 AS keycloak-build
ARG KEYCLOAK_DB
COPY --from=keycloak-providers --chown=1000:0 /providers/ /opt/keycloak/providers/
COPY --chown=1000:0 deploy/keycloak/conf/keycloak-${KEYCLOAK_DB}.conf /opt/keycloak/conf/keycloak.conf
COPY --chown=1000:0 deploy/keycloak/conf/quarkus.properties /opt/keycloak/conf/quarkus.properties
RUN if [ "$KEYCLOAK_DB" != sqlite ]; then rm -f /opt/keycloak/providers/sqlite-jdbc.jar /opt/keycloak/providers/mesail-keycloak-sqlite.jar; fi \
  && /opt/keycloak/bin/kc.sh build \
  && /opt/keycloak/bin/kc.sh show-config | grep -F "kc.db "

FROM quay.io/keycloak/keycloak:26.8.2@sha256:723b1d6e254c0fc96d5cffec860c7e0744d6a3277822b865bb538cf891575e41 AS mesail-keycloak
ARG KEYCLOAK_DB
COPY --from=keycloak-build /opt/keycloak/ /opt/keycloak/
COPY --chown=0:0 deploy/keycloak/realm /opt/mesail/keycloak/realm
COPY --chown=0:0 --chmod=755 deploy/keycloak/bin /opt/mesail/keycloak/bin
USER 0
RUN mkdir -p /data/keycloak && chown 1000:0 /data/keycloak && chmod 0770 /data/keycloak
USER 1000
ENV MESAIL_KEYCLOAK_DB=${KEYCLOAK_DB} \
    MESAIL_KEYCLOAK_DATA=/data/keycloak
VOLUME ["/data/keycloak"]
HEALTHCHECK --interval=15s --timeout=8s --start-period=180s --retries=3 \
  CMD ["/opt/mesail/keycloak/bin/mesail-healthcheck"]
ENTRYPOINT ["/opt/mesail/keycloak/bin/mesail-supervisor"]


FROM ${LAMBDA_ADAPTER_IMAGE} AS lambda-adapter

FROM ${UBUNTU_IMAGE} AS lambda-tools
ARG GO_VERSION=1.27.3
ARG GO_SHA256_ARM64=a06e99309dbcbdca1a8ccd2282c3ee146e55930886e26b4f1193269587165f49
ARG GO_SHA256_AMD64=65efbc80fdc1fa2cd2631cad5b481777a68e8725f34e89f50fc6ac714e379b40
ARG TACHYON_VERSION=26.46.02
ARG TY_SHA256_ARM64=3006c7bcd1bd7e879055d60492b307c2903dde61ec72bb36950ace7ff3031b2e
ARG TY_SHA256_AMD64=125423c88dac3e888cc442c807f1beffc10ff8e5db2ac4cd70b331ecda5ea1fd
ARG FYLO_VERSION=26.45.07
ARG FYLO_SHA256_ARM64=45b83a0211a45bc3d50ee487867513c2c135b53bc914c728f78311f8b712667c
ARG FYLO_SHA256_AMD64=c198bd3cb9651578756229eddde732ab810422c6ba5fc4a3cb4678d542bdc1cd
ENV DEBIAN_FRONTEND=noninteractive LANG=C.UTF-8 PATH=/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
COPY --chmod=755 deploy/lambda/install-tools.sh /usr/local/lib/mesail/install-tools.sh
RUN set -eu \
 && apt-get update -q \
 && apt-get install -y -q --no-install-recommends ca-certificates curl tzdata jq \
 && /usr/local/lib/mesail/install-tools.sh \
 && apt-get purge -y -q curl \
 && apt-get autoremove -y -q \
 && rm -rf /var/lib/apt/lists/* /var/cache/apt/* /var/log/apt /var/log/dpkg.log /tmp/*

FROM lambda-tools AS lambda
ARG APP
ARG FYLO=false
ARG ISSUER_ORIGIN=https://mesail.localhost
ARG KEYCLOAK_URL=http://keycloak:8080/idp
ARG SERVICE_BASE=http://router:8080
ARG USER_ID=10001
ENV MESAIL_APP_ROOT=/tmp/mesail-app \
    MESAIL_STORED_APP=/opt/mesail/app \
    MESAIL_STORED_CACHE=/opt/mesail/cache \
    MESAIL_USER_ID=${USER_ID} \
    HOME=/tmp TMPDIR=/tmp \
    TAC_CACHE_DIR=/tmp/tac-cache \
    GOTOOLCHAIN=local GOPROXY=off GOFLAGS=-buildvcs=false GOCACHE=/tmp/go-build GOPATH=/tmp/go
RUN test -n "${APP}" \
 && useradd --system --no-create-home --home-dir /tmp --uid "${USER_ID}" --user-group mesail
COPY --from=lambda-adapter /lambda-adapter /opt/extensions/lambda-adapter
COPY --chmod=755 deploy/lambda/start.sh /usr/local/bin/mesail-lambda-start
COPY --chmod=755 deploy/lambda/write-profile.sh /usr/local/lib/mesail/write-profile.sh
COPY --chmod=755 deploy/lambda/rewrite-config.sh /usr/local/lib/mesail/rewrite-config.sh
COPY apps/${APP} /tmp/mesail-app
RUN set -eu \
 && mkdir -p /opt/mesail /tmp/tac-cache \
 && /usr/local/lib/mesail/rewrite-config.sh /tmp/mesail-app/server/application.json "${ISSUER_ORIGIN}" "${KEYCLOAK_URL}" "${SERVICE_BASE}" \
 && ty bundle /tmp/mesail-app \
 && /usr/local/lib/mesail/write-profile.sh /tmp/mesail-app "${APP}" "${FYLO}" > /opt/mesail/data-root \
 && mv /tmp/mesail-app /opt/mesail/app \
 && mv /tmp/tac-cache /opt/mesail/cache \
 && rm -rf /tmp/* /tmp/.cache \
 && chmod -R a+rX /opt/mesail
ENV YON_PROFILE=lambda \
    YON_EGRESS_ENFORCEMENT=proxy \
    MESAIL_SERVICE=${APP} \
    AWS_LWA_PORT=8080 \
    AWS_LWA_READINESS_CHECK_PATH=/api/v1/${APP}/health \
    AWS_LWA_READINESS_CHECK_PROTOCOL=http
ENTRYPOINT ["/usr/local/bin/mesail-lambda-start"]

FROM ${GOLANG_IMAGE} AS router-build
WORKDIR /src/deploy/router
COPY kit /src/kit
COPY deploy/router/go.mod ./
COPY deploy/router/*.go ./
RUN rm -f ./*_test.go \
 && GOTOOLCHAIN=local GOPROXY=off CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/mesail-router .

FROM curlimages/curl:8.11.1@sha256:c1fe1679c34d9784c1b0d1e5f62ac0a79fca01fb6377cdd33e90473c6f9f9a69 AS scheduler
COPY --chmod=755 deploy/lambda/scheduler.sh /usr/local/bin/mesail-scheduler
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/mesail-scheduler"]

FROM scratch AS router
COPY --from=lambda-tools /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=router-build /out/mesail-router /mesail-router
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/mesail-router"]

FROM ${GOLANG_IMAGE} AS realtime-gateway-build
WORKDIR /src/edge/realtime
COPY edge/realtime/go.mod edge/realtime/go.sum ./
RUN GOTOOLCHAIN=local go mod download -x
COPY edge/realtime/ ./
RUN GOTOOLCHAIN=local GOFLAGS=-mod=readonly CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/mesail-realtime-gateway .

FROM lambda-tools AS realtime
ARG USER_ID=10001
ENV MESAIL_APP_ROOT=/opt/mesail/realtime \
    MESAIL_DATA_ROOT=/data/realtime \
    HOME=/tmp TMPDIR=/tmp \
    TAC_CACHE_DIR=/opt/mesail/cache \
    GOTOOLCHAIN=local GOPROXY=off GOFLAGS=-buildvcs=false GOCACHE=/tmp/go-build GOPATH=/tmp/go
RUN useradd --system --no-create-home --home-dir /tmp --uid "${USER_ID}" --user-group mesail \
 && mkdir -p /opt/mesail/realtime /opt/mesail/cache /data \
 && chown "${USER_ID}:${USER_ID}" /opt/mesail/realtime /opt/mesail/cache /data
COPY --from=realtime-gateway-build /out/mesail-realtime-gateway /usr/local/bin/mesail-realtime-gateway
COPY --chmod=755 deploy/lambda/realtime-start.sh /usr/local/bin/mesail-realtime-start
COPY --chown=${USER_ID}:${USER_ID} apps/realtime /opt/mesail/realtime
USER ${USER_ID}:${USER_ID}
RUN set -eu \
 && printf '{ "server": { "hosts": ["realtime"] } }\n' > /opt/mesail/realtime/server/application-container.json \
 && ty bundle /opt/mesail/realtime \
 && rm -rf /tmp/* /tmp/.cache
ENV YON_PROFILE=container \
    YON_EGRESS_ENFORCEMENT=proxy \
    MESAIL_REALTIME_LISTEN=0.0.0.0:8116
EXPOSE 8106 8116
ENTRYPOINT ["/usr/local/bin/mesail-realtime-start"]

FROM mesail-keycloak AS keycloak-blackbox
COPY --chown=0:0 deploy/floci/keycloak-users/ /opt/mesail/keycloak/realm/dev/
