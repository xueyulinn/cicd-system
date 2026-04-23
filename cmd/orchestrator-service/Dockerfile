FROM golang:1.25-alpine AS builder

WORKDIR /src

# Cache dependencies first for faster rebuilds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /out/orchestrator-service ./cmd/orchestrator-service

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/orchestrator-service /app/orchestrator-service

EXPOSE 8002

ENTRYPOINT ["/app/orchestrator-service"]
