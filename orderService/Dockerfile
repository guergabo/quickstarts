# Stage 1: Get Go image.
FROM docker.io/library/golang:1.23.3-bookworm AS builder

# Add working directory.
WORKDIR /order

# Add source code.
RUN mkdir -p ./src/antithesis/order/db/ops
COPY go.mod ./src/antithesis/order/
COPY *.go ./src/antithesis/order/
COPY db/ ./src/antithesis/order/db/

# Download and install instrumentor.
RUN cd ./src/antithesis/order && \
go install github.com/antithesishq/antithesis-sdk-go/tools/antithesis-go-instrumentor@v0.4.0 && \
go mod tidy

# Create the destination for instrumented code.
RUN mkdir -p ./src/antithesis/order-instrumented

# Perform instrumentation.
RUN /go/bin/antithesis-go-instrumentor \
./src/antithesis/order \
./src/antithesis/order-instrumented

# Build order binary.
RUN cd ./src/antithesis/order-instrumented/customer && \
go build -o order *.go && ls -la

RUN cd ./src/antithesis/order-instrumented && ls -la

# Stage 2: lightweight "release"
FROM docker.io/library/debian:bookworm-slim

# Copy the instrumented binary and symbols from the build image.
COPY --from=builder \
/order/src/antithesis/order-instrumented/customer/order /bin/
RUN mkdir -p /symbols
COPY --from=builder \
/order/src/antithesis/order-instrumented/symbols /symbols/

ENTRYPOINT [ "/bin/order" ]
