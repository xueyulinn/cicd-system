FROM golang:1.25-alpine AS builder

WORKDIR /src

# Cache dependencies first for faster rebuilds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /out/api-gateway ./cmd/api-gateway

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/api-gateway /app/api-gateway

EXPOSE 8000

ENTRYPOINT ["/app/api-gateway"]
