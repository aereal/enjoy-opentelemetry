FROM ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector-contrib:0.60.0

ADD ./collector.yml ./app-collector.yml
CMD ["--config", "./app-collector.yml"]
