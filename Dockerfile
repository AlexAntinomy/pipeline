From golang:1.16 AS builder
WORKDIR /app
COPY . .
RUN go build -o myapp .

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/myapp .
CMD ["./myapp"]