#!/bin/bash
echo "Waiting for Kafka to be ready..."
sleep 5

KAFKA_BIN="/opt/kafka/bin"
BROKER="localhost:9092"

for topic in telemetry.raw events.kills alerts.detections; do
    $KAFKA_BIN/kafka-topics.sh \
        --bootstrap-server $BROKER \
        --create \
        --if-not-exists \
        --topic $topic \
        --partitions 4 \
        --replication-factor 1
    echo "Created topic: $topic"
done

echo "All topics created."
