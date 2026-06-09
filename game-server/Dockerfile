FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o game-server .

FROM alpine:3.19
COPY --from=builder /app/game-server /usr/local/bin/
EXPOSE 8080
CMD ["game-server"]
