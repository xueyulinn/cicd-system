FROM golang:1.25-alpine AS builder

WORKDIR /src

# Cache dependencies first for faster rebuilds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/validation-service ./cmd/validation-service

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/validation-service /app/validation-service

EXPOSE 8001

ENTRYPOINT ["/app/validation-service"]
