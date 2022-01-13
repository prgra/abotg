FROM golang:1.17 AS builder
COPY main.go go.mod go.sum /app/
COPY abot /app/abot
WORKDIR /app
RUN go build

FROM alpine:latest 
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/abotg /app/
CMD ["/app/abotg"]  