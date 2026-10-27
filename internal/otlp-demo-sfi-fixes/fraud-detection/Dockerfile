# Re-bases the upstream OpenTelemetry demo fraud-detection service onto the
# Microsoft Build of OpenJDK 17 distroless image (Azure Linux 3.0) from MCR.
# The upstream base (gcr.io/distroless/java17-debian12) ships libbrotli1
# 1.0.9-2+b6 (CVE-2026-111497); the MCR base ships no brotli library.
ARG SOURCE_IMAGE=rashmiotlp0e4773a2.azurecr.io/open-telemetry/demo:3.0.0-fraud-detection
FROM ${SOURCE_IMAGE} AS source

FROM mcr.microsoft.com/openjdk/jdk:17-distroless
WORKDIR /usr/src/app/
# Only the app jar and the OTel Java agent are taken from the upstream image.
COPY --from=source /usr/src/app/ /usr/src/app/
ENV JAVA_TOOL_OPTIONS="-javaagent:/usr/src/app/opentelemetry-javaagent.jar -Xmx180m"
# Non-root "app" user that ships with the MCR OpenJDK image.
USER 101
ENTRYPOINT ["/usr/jdk/bin/java", "-jar", "fraud-detection-1.0-all.jar"]
