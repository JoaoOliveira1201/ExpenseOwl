# syntax=docker/dockerfile:1
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/expenseowl ./cmd/expenseowl

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && addgroup -S expenseowl && adduser -S -G expenseowl expenseowl
WORKDIR /app
RUN mkdir -p /app/data/receipts && chown -R expenseowl:expenseowl /app
COPY --from=build /out/expenseowl /usr/local/bin/expenseowl
USER expenseowl
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["expenseowl"]
