# Stage 1: Get Go image.
FROM docker.io/library/golang:1.23.3-bookworm AS builder

# Add working directory.
WORKDIR /workload

# Add source code.
RUN mkdir -p ./src/antithesis/workload
COPY go.mod go.sum *.go ./src/antithesis/workload

# Download and install instrumentor.
RUN cd ./src/antithesis/workload && \
go install github.com/antithesishq/antithesis-sdk-go/tools/antithesis-go-instrumentor@v0.4.0 && \
go mod tidy

# Create the destination for instrumented code.
RUN mkdir -p ./src/antithesis/workload-instrumented

# Perform instrumentation.
RUN /go/bin/antithesis-go-instrumentor \
./src/antithesis/workload \
./src/antithesis/workload-instrumented

# Build workload binary.
RUN cd ./src/antithesis/workload-instrumented/customer && \
go build -o workload *.go && ls -la

RUN cd ./src/antithesis/workload-instrumented && ls -la

# Stage 2: lightweight "release"
FROM docker.io/library/debian:bookworm-slim

# Copy the instrumented binary and symbols from the build image.
COPY --from=builder \
/workload/src/antithesis/workload-instrumented/customer/workload /bin/
RUN mkdir -p /symbols
COPY --from=builder \
/workload/src/antithesis/workload-instrumented/symbols /symbols/

ENTRYPOINT [ "/bin/workload" ]
