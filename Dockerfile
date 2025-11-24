FROM golang:1.25 AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/redbook ./cmd

FROM gcr.io/distroless/base-debian12
ENV GIN_MODE=release
WORKDIR /app

COPY --from=builder /bin/redbook /usr/local/bin/redbook
COPY config.yaml /app/config.yaml

EXPOSE 8080
CMD ["redbook"]

