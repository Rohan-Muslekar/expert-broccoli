# Infrastructure

Configuration files for Kafka, Prometheus, and Grafana. These services are managed via the root `docker-compose.yml`.

## Kafka

Apache Kafka 3.7.0 running in KRaft mode (no ZooKeeper).

**Topics** (created by `kafka/create-topics.sh` on startup):

| Topic | Partitions | Description |
|-------|-----------|-------------|
| `telemetry.raw` | 4 | Raw player telemetry per tick |
| `events.kills` | 4 | Kill events with reaction time |
| `features.computed` | 4 | 18-feature behavioral vectors |
| `alerts.detections` | 4 | Cheat detection alerts |

**Useful commands:**

```bash
# List topics
make kafka-topics

# Consume from a topic
docker compose exec kafka /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic alerts.detections \
  --from-beginning

# Describe a topic
docker compose exec kafka /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server localhost:9092 \
  --describe --topic features.computed
```

## Prometheus

Prometheus 2.51.0 scraping metrics from the game server and ML service every 5 seconds.

**Scrape targets** (configured in `prometheus/prometheus.yml`):

| Target | Endpoint | Metrics |
|--------|----------|---------|
| game-server | `:8080/metrics` | Tick duration, active players, Kafka publish rates, feature engine stats, rule-based alert counts |
| ml-service | `:8000/metrics` | Model loaded status, prediction counts, alert counts, inference durations, anomaly scores |

**Access:** http://localhost:9091

## Grafana

Grafana 10.4.0 with auto-provisioned dashboards and Prometheus data source.

**Dashboards** (in `grafana/dashboards/`):

| Dashboard | Description |
|-----------|-------------|
| Pipeline Health | Game tick rate, active players, Kafka publish latency, feature processing duration |
| Detection Analytics | Alerts by rule/model, prediction confidence distributions, anomaly score histograms |

**Access:** http://localhost:3000

## Directory Structure

```
infra/
  kafka/
    create-topics.sh                          # Topic initialization script
  prometheus/
    prometheus.yml                            # Scrape configuration
  grafana/
    provisioning/
      datasources/prometheus.yml              # Auto-provision Prometheus
      dashboards/dashboards.yml               # Dashboard auto-loading config
    dashboards/
      pipeline-health.json                    # Pipeline health dashboard
      detection-analytics.json                # Detection analytics dashboard
```
