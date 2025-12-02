# Observability Examples for MCP DevTools

This directory contains example configurations for setting up observability with MCP DevTools using OpenTelemetry.

- [Observability Examples for MCP DevTools](#observability-examples-for-mcp-devtools)
  - [Quick Start - Jaeger Only (Recommended for just tracing)](#quick-start---jaeger-only-recommended-for-just-tracing)
  - [Configuration Scenarios](#configuration-scenarios)
  - [Configuration Files](#configuration-files)
  - [Common Operations](#common-operations)
  - [Troubleshooting](#troubleshooting)
  - [Production Considerations](#production-considerations)
  - [Example Queries](#example-queries)
  - [Further Reading](#further-reading)

## Quick Start - Jaeger Only (Recommended for just tracing)

The simplest way to get started with distributed tracing, no config files needed:

```bash
cd docs/observability/examples
docker compose up -d
```

Configure MCP DevTools:
```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 ./bin/mcp-devtools
```

View traces: **http://localhost:16686**

## Configuration Scenarios

### 1. Jaeger Only (Default)

Simple distributed tracing with persistent storage. Jaeger has a built-in OTLP receiver, so no OTEL Collector is needed.

**What you get:**
- Distributed tracing with Jaeger UI
- 7 days trace retention
- Direct OTLP ingestion from MCP DevTools

**Setup:**
```bash
docker compose up -d
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 ./bin/mcp-devtools
```

**Access:**
- Jaeger UI: http://localhost:16686

**Note:** This setup does NOT use the OTEL Collector - Jaeger receives traces directly.

---

### 2. Jaeger + Prometheus + Grafana (advanced)

Full observability stack with traces and metrics.

**What you get:**
- Distributed tracing via Jaeger
- Metrics collection via Prometheus
- Unified visualisation in Grafana
- 30 days metric retention

**Setup:**

1. Edit `docker-compose.yaml`:
   - Uncomment `otel-collector` service
   - Uncomment `prometheus` service
   - Uncomment `grafana` service
   - Uncomment volume definitions

2. Update OTEL collector config mount:
   ```yaml
   - ./configs/otel/jaeger-prometheus.yaml:/etc/otel-collector-config.yaml:ro
   ```

3. Start services:
   ```bash
   docker compose up -d
   ```

4. Configure MCP DevTools:
   ```bash
   OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 \
   MCP_METRICS_GROUPS=tool,session,cache,security \
   ./bin/mcp-devtools
   ```

**Access:**
- Jaeger UI: http://localhost:16686
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)

**Grafana Configuration:**
- Datasources are auto-provisioned (Prometheus + Jaeger)
- Create custom dashboards or import community dashboards
- Explore traces via Jaeger datasource
- Query metrics via Prometheus datasource

---

### 3. AWS X-Ray

Export traces to AWS X-Ray for cloud-native observability.

**What you get:**
- Traces sent to AWS X-Ray service
- CloudWatch integration
- AWS service map
- Managed trace storage

**Prerequisites:**
```bash
export AWS_ACCESS_KEY_ID=your_key
export AWS_SECRET_ACCESS_KEY=your_secret
export AWS_REGION=us-east-1
```

**Setup:**

1. Edit `docker-compose.yaml`:
   - Comment out `jaeger` service
   - Uncomment `otel-collector` service

2. Update OTEL collector config mount:
   ```yaml
   - ./configs/otel/xray.yaml:/etc/otel-collector-config.yaml:ro
   ```

3. Add AWS credentials to `otel-collector` service:
   ```yaml
   environment:
     - AWS_ACCESS_KEY_ID=${AWS_ACCESS_KEY_ID}
     - AWS_SECRET_ACCESS_KEY=${AWS_SECRET_ACCESS_KEY}
     - AWS_REGION=${AWS_REGION}
   ```

4. Start services:
   ```bash
   docker compose up -d
   ```

5. Configure MCP DevTools:
   ```bash
   OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 \
   OTEL_SERVICE_NAME=mcp-devtools \
   ./bin/mcp-devtools
   ```

**Access:**
- AWS Console: https://console.aws.amazon.com/xray/home
- Filter by service name: "mcp-devtools"

---

### 4. Grafana Tempo

Lightweight tracing backend optimised for cost and simplicity.

**What you get:**
- S3-compatible object storage for traces
- Native Grafana integration
- Lower resource usage than Jaeger
- TraceQL query language

**Setup:**

1. Edit `docker-compose.yaml`:
   - Comment out `jaeger` service
   - Uncomment `tempo` service
   - Uncomment `grafana` service
   - Uncomment volume definitions

2. Update Grafana datasources config to use Tempo:
   - Edit `configs/grafana/datasources/datasources.yaml`
   - Comment out Jaeger datasource
   - Uncomment Tempo datasource

3. Start services:
   ```bash
   docker compose up -d
   ```

4. Configure MCP DevTools:
   ```bash
   OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 ./bin/mcp-devtools
   ```

**Access:**
- Grafana: http://localhost:3000 (admin/admin)
- Explore → Tempo → Search

---

## Configuration Files

All configuration files are organised in `configs/` by service:

```
configs/
├── grafana/
│   ├── datasources/datasources.yaml    # Auto-provisioned datasources
│   └── dashboards/dashboards.yaml      # Dashboard provisioning
├── jaeger/
│   └── jaeger.yaml                     # Commented OTEL Collector config (reference)
├── otel/
│   ├── jaeger-prometheus.yaml          # OTEL → Jaeger + Prometheus
│   └── xray.yaml                       # OTEL → AWS X-Ray
├── prometheus/
│   └── prometheus.yml                  # Prometheus scrape config
└── tempo/
    └── tempo.yaml                      # Tempo configuration
```

### Configuration Details

| Config | Purpose | When to Use |
|--------|---------|-------------|
| `otel/jaeger-prometheus.yaml` | Routes traces to Jaeger, metrics to Prometheus | Full observability stack |
| `otel/xray.yaml` | Exports traces to AWS X-Ray | AWS cloud deployments |
| `jaeger/jaeger.yaml` | Reference OTEL config (commented) | Learning/documentation |
| `tempo/tempo.yaml` | Direct Tempo configuration | Using Tempo instead of Jaeger |
| `prometheus/prometheus.yml` | Scrapes OTEL Collector metrics | Metrics collection |
| `grafana/datasources/` | Auto-configures datasources | Grafana setup |

---

## Common Operations

### View Logs
```bash
# All services
docker compose logs -f

# Specific service
docker compose logs -f jaeger
docker compose logs -f otel-collector
```

### Restart Services
```bash
# All services
docker compose restart

# Specific service
docker compose restart otel-collector
```

### Clean Up
```bash
# Stop and remove containers
docker compose down

# Also remove volumes (deletes all data)
docker compose down -v
```

### Update Configuration
After editing config files:
```bash
docker compose restart otel-collector
```

---

## Troubleshooting

### No Traces Appearing

**Check OTEL endpoint:**
```bash
curl http://localhost:4318/v1/traces  # Should return 405
```

**Check container logs:**
```bash
docker compose logs jaeger
docker compose logs otel-collector
```

**Verify MCP DevTools config:**
```bash
echo $OTEL_EXPORTER_OTLP_ENDPOINT
```

### No Metrics in Prometheus

**Check OTEL Collector is exposing metrics:**
```bash
curl http://localhost:8889/metrics | grep mcp_
```

**Check Prometheus targets:**
- Open http://localhost:9090/targets
- Verify `otel-collector` target is UP

**Verify metrics are enabled:**
```bash
echo $MCP_METRICS_GROUPS  # Should show: tool,session,cache,security
```

### Grafana Datasources Not Working

**Check datasource provisioning:**
```bash
docker compose logs grafana | grep datasource
```

**Verify network connectivity:**
```bash
docker compose exec grafana wget -O- http://prometheus:9090/-/healthy
docker compose exec grafana wget -O- http://jaeger:16686
```

### High Memory Usage

**Reduce OTEL Collector memory:**
Edit the collector config's `memory_limiter` processor:
```yaml
memory_limiter:
  limit_mib: 256  # Reduce from 512
  spike_limit_mib: 64  # Reduce from 128
```

**Enable sampling:**
```bash
OTEL_TRACES_SAMPLER=traceidratio \
OTEL_TRACES_SAMPLER_ARG=0.1 \
./bin/mcp-devtools
```

---

## Production Considerations

### Security

- **Enable authentication** on Grafana, Prometheus, Jaeger
- **Use TLS** for OTLP endpoints (configure in OTEL Collector)
- **Restrict network access** using Docker networks and firewalls
- **Rotate credentials** for Grafana and external services

### Persistence

All services use named volumes for data persistence:
- `jaeger-data` - Trace storage (Badger)
- `prometheus-data` - Metrics (TSDB)
- `grafana-data` - Dashboards and config
- `tempo-data` - Trace storage

### Retention

**Jaeger:**
- Default: 7 days (`BADGER_SPAN_STORE_TTL=168h`)
- Adjust in `docker-compose.yaml`

**Prometheus:**
- Default: 30 days (`--storage.tsdb.retention.time=30d`)
- Adjust in `docker-compose.yaml`

**Tempo:**
- Default: No expiration (manual cleanup required)
- Configure retention in `configs/tempo.yaml`

### Resource Limits

Add resource limits in `docker-compose.yaml`:
```yaml
services:
  jaeger:
    deploy:
      resources:
        limits:
          memory: 1G
          cpus: '1.0'
```

---

## Example Queries

### Prometheus (PromQL)

```promql
# Tool call rate (calls/second)
rate(mcp_tool_calls_total[5m])

# P95 latency by tool
histogram_quantile(0.95, rate(mcp_tool_duration_seconds_bucket[5m]))

# Error rate
sum(rate(mcp_tool_errors_total[5m])) / sum(rate(mcp_tool_calls_total[5m]))

# Cache hit ratio
sum(rate(mcp_cache_operations_total{result="hit"}[5m])) /
sum(rate(mcp_cache_operations_total{operation="get"}[5m]))
```

### Jaeger (Search)

```
service="mcp-devtools" AND mcp.tool.name="internet_search"
service="mcp-devtools" AND http.status_code >= 500
service="mcp-devtools" AND mcp.session.id="abc123"
```

---

## Further Reading

- [Complete Observability Documentation](../observability.md)
- [OpenTelemetry Collector Docs](https://opentelemetry.io/docs/collector/)
- [Jaeger Documentation](https://www.jaegertracing.io/docs/)
- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)
- [Grafana Tempo Documentation](https://grafana.com/docs/tempo/)
- [AWS X-Ray Documentation](https://docs.aws.amazon.com/xray/)
