```
docker run \
  -d \ # daemonize
  --rm --hostname $(hostname) \
  -e DD_API_KEY=$DD_API_KEY \
  -p 4318:4318 \
  -v $(pwd)/collector.yml:/etc/otelcol-contrib/config.yaml \
  ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector-contrib:0.60.0
```
