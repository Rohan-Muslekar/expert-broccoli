package telemetry

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	enabled         bool
	telemetryWriter *kafka.Writer
	killsWriter     *kafka.Writer
}

func NewProducer(enabled bool, brokers []string) *Producer {
	p := &Producer{enabled: enabled}
	if !enabled {
		return p
	}

	p.telemetryWriter = &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        "telemetry.raw",
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 5 * time.Millisecond,
		Async:        true,
	}

	p.killsWriter = &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        "events.kills",
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 5 * time.Millisecond,
		Async:        true,
	}

	return p
}

func (p *Producer) PublishTelemetry(ctx context.Context, t PlayerTelemetry) {
	data, err := json.Marshal(t)
	if err != nil {
		log.Printf("marshal telemetry error: %v", err)
		return
	}

	if !p.enabled {
		log.Printf("[telemetry] %s", data)
		return
	}

	err = p.telemetryWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(t.PlayerID),
		Value: data,
	})
	if err != nil {
		log.Printf("kafka telemetry write error: %v", err)
	}
}

func (p *Producer) PublishKill(ctx context.Context, k KillEvent) {
	data, err := json.Marshal(k)
	if err != nil {
		log.Printf("marshal kill error: %v", err)
		return
	}

	if !p.enabled {
		log.Printf("[kill] %s", data)
		return
	}

	err = p.killsWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(k.KillerID),
		Value: data,
	})
	if err != nil {
		log.Printf("kafka kill write error: %v", err)
	}
}

func (p *Producer) Close() {
	if p.telemetryWriter != nil {
		p.telemetryWriter.Close()
	}
	if p.killsWriter != nil {
		p.killsWriter.Close()
	}
}
