package engine

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"cheat-detection/feature-engine/metrics"
	"github.com/segmentio/kafka-go"
)

// EvaluateFunc evaluates a feature vector and returns any triggered alerts.
type EvaluateFunc func(fv FeatureVector) []AlertEvent

type Pipeline struct {
	reader         *kafka.Reader
	featureWriter  *kafka.Writer
	alertWriter    *kafka.Writer
	playerTimeout  time.Duration
	processors     map[string]*playerSlot
	mu             sync.Mutex
	evaluate       EvaluateFunc
}

type playerSlot struct {
	proc     *PlayerProcessor
	lastSeen time.Time
}

func NewPipeline(brokers []string, consumeTopic, consumerGroup, produceTopic, alertsTopic string, playerTimeoutSec int, evaluate EvaluateFunc) *Pipeline {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    consumeTopic,
		GroupID:  consumerGroup,
		MinBytes: 1,
		MaxBytes: 10e6,
		MaxWait:  100 * time.Millisecond,
	})

	featureWriter := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        produceTopic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 5 * time.Millisecond,
		Async:        true,
	}

	alertWriter := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        alertsTopic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 5 * time.Millisecond,
		Async:        true,
	}

	return &Pipeline{
		reader:        reader,
		featureWriter: featureWriter,
		alertWriter:   alertWriter,
		playerTimeout: time.Duration(playerTimeoutSec) * time.Second,
		processors:    make(map[string]*playerSlot),
		evaluate:      evaluate,
	}
}

func (p *Pipeline) Run(ctx context.Context) {
	go p.cleanupLoop(ctx)

	for {
		msg, err := p.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("fetch error: %v", err)
			continue
		}

		metrics.EventsConsumed.Inc()

		var ev PlayerTelemetry
		if err := json.Unmarshal(msg.Value, &ev); err != nil {
			log.Printf("unmarshal error: %v", err)
			p.reader.CommitMessages(ctx, msg)
			continue
		}

		start := time.Now()
		p.processEvent(ctx, ev)
		metrics.ProcessingDuration.Observe(time.Since(start).Seconds())

		p.reader.CommitMessages(ctx, msg)
	}
}

func (p *Pipeline) processEvent(ctx context.Context, ev PlayerTelemetry) {
	p.mu.Lock()
	slot, exists := p.processors[ev.PlayerID]
	if !exists {
		slot = &playerSlot{
			proc: NewPlayerProcessor(ev.PlayerID),
		}
		p.processors[ev.PlayerID] = slot
		metrics.ActivePlayers.Inc()
		log.Printf("new player processor: %s", ev.PlayerID)
	}
	slot.lastSeen = time.Now()
	p.mu.Unlock()

	fv := slot.proc.Process(ev)

	fvData, err := json.Marshal(fv)
	if err != nil {
		log.Printf("marshal feature error: %v", err)
		return
	}
	err = p.featureWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(fv.PlayerID),
		Value: fvData,
	})
	if err != nil {
		log.Printf("feature publish error: %v", err)
	} else {
		metrics.FeaturesPublished.Inc()
	}

	alerts := p.evaluate(fv)
	for _, alert := range alerts {
		alertData, err := json.Marshal(alert)
		if err != nil {
			log.Printf("marshal alert error: %v", err)
			continue
		}
		err = p.alertWriter.WriteMessages(ctx, kafka.Message{
			Key:   []byte(alert.PlayerID),
			Value: alertData,
		})
		if err != nil {
			log.Printf("alert publish error: %v", err)
		} else {
			metrics.AlertsFired.WithLabelValues(alert.Rule).Inc()
		}
	}
}

func (p *Pipeline) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.mu.Lock()
			now := time.Now()
			for id, slot := range p.processors {
				if now.Sub(slot.lastSeen) > p.playerTimeout {
					delete(p.processors, id)
					metrics.ActivePlayers.Dec()
					log.Printf("cleaned up player processor: %s", id)
				}
			}
			p.mu.Unlock()
		}
	}
}

func (p *Pipeline) Close() {
	p.reader.Close()
	p.featureWriter.Close()
	p.alertWriter.Close()
}
