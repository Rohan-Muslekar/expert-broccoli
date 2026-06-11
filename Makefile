.PHONY: up down logs build run-local clean kafka-topics

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

build:
	cd game-server && go build -o game-server .

run-local:
	cd game-server && KAFKA_ENABLED=false ./game-server

kafka-topics:
	docker compose exec kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --list

clean:
	cd game-server && rm -f game-server
	docker compose down -v
