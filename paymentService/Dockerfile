# Stage 1: Get Go image.
FROM docker.io/library/golang:1.23.3-bookworm AS builder

# Add working directory.
WORKDIR /payment

# Add source code.
RUN mkdir -p ./src/antithesis/payment
COPY go.mod *.go ./src/antithesis/payment

# Download and install instrumentor.
RUN cd ./src/antithesis/payment && \
go install github.com/antithesishq/antithesis-sdk-go/tools/antithesis-go-instrumentor@v0.4.0 && \
go mod tidy

# Create the destination for instrumented code.
RUN mkdir -p ./src/antithesis/payment-instrumented

# Perform instrumentation.
RUN /go/bin/antithesis-go-instrumentor \
./src/antithesis/payment \
./src/antithesis/payment-instrumented

# Build payment binary.
RUN cd ./src/antithesis/payment-instrumented/customer && \
go build -o payment *.go && ls -la

RUN cd ./src/antithesis/payment-instrumented && ls -la

# Stage 2: lightweight "release"
FROM docker.io/library/debian:bookworm-slim

# Copy the instrumented binary and symbols from the build image.
COPY --from=builder \
/payment/src/antithesis/payment-instrumented/customer/payment /bin/
RUN mkdir -p /symbols
COPY --from=builder \
/payment/src/antithesis/payment-instrumented/symbols /symbols/

ENTRYPOINT [ "/bin/payment" ]
