FROM golang:1.22 AS builder
WORKDIR /app
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -o taskctl .

FROM scratch
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /app/taskctl /taskctl
ENV TZ=UTC
ENTRYPOINT ["/taskctl"]
