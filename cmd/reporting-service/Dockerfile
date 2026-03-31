FROM golang:1.25-alpine AS builder

WORKDIR /src

# Cache dependencies first for faster rebuilds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /out/reporting-service ./cmd/reporting-service

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/reporting-service /app/reporting-service

EXPOSE 8004

ENTRYPOINT ["/app/reporting-service"]
