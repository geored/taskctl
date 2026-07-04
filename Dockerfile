FROM golang:1.22 AS builder
WORKDIR /app
COPY go.mod ./
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
# BUILD_DATE maps to the Go variable main.date via -ldflags -X.
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE}" -o taskctl .

FROM scratch
COPY --from=builder /app/taskctl /taskctl
ENTRYPOINT ["/taskctl"]
