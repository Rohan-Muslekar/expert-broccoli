FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o feature-engine .

FROM alpine:3.19
COPY --from=builder /app/feature-engine /usr/local/bin/
EXPOSE 9090
CMD ["feature-engine"]
