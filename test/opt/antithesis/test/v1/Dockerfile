# Stage 2: building commands.
FROM docker.io/library/golang:1.23.3-bookworm AS builder

# Add working directory.
WORKDIR /commands

# Create source directories for each binary.
RUN mkdir -p ./src/antithesis/commands/basic
RUN mkdir -p ./src/antithesis/commands/intermediate/finally_consistent_data
RUN mkdir -p ./src/antithesis/commands/intermediate/parallel_driver_writes

# Copy source files for each command.
COPY basic/go.mod basic/go.sum basic/*.go ./src/antithesis/commands/basic/
COPY intermediate/finally_consistent_data/go.mod intermediate/finally_consistent_data/go.sum intermediate/finally_consistent_data/*.go ./src/antithesis/commands/intermediate/finally_consistent_data/
COPY intermediate/parallel_driver_writes/go.mod intermediate/parallel_driver_writes/go.sum intermediate/parallel_driver_writes/*.go ./src/antithesis/commands/intermediate/parallel_driver_writes/

# Download and install instrumentor.
RUN cd ./src/antithesis/commands && \
    go install github.com/antithesishq/antithesis-sdk-go/tools/antithesis-go-instrumentor@v0.4.0 

# Create the destination directories for instrumented code
RUN mkdir -p ./src/antithesis/commands-instrumented/basic
RUN mkdir -p ./src/antithesis/commands-instrumented/intermediate/finally_consistent_data
RUN mkdir -p ./src/antithesis/commands-instrumented/intermediate/parallel_driver_writes

# Perform instrumentation for each component
RUN /go/bin/antithesis-go-instrumentor \
    ./src/antithesis/commands/basic \
    ./src/antithesis/commands-instrumented/basic

RUN /go/bin/antithesis-go-instrumentor \
    ./src/antithesis/commands/intermediate/finally_consistent_data \
    ./src/antithesis/commands-instrumented/intermediate/finally_consistent_data

RUN /go/bin/antithesis-go-instrumentor \
    ./src/antithesis/commands/intermediate/parallel_driver_writes \
    ./src/antithesis/commands-instrumented/intermediate/parallel_driver_writes

# Build each instrumented binary
RUN cd ./src/antithesis/commands-instrumented/basic/customer && \
    go build -o singleton_driver_basic *.go && ls -la

RUN cd ./src/antithesis/commands-instrumented/intermediate/finally_consistent_data/customer && \
    go build -o finally_consistent_data *.go && ls -la

RUN cd ./src/antithesis/commands-instrumented/intermediate/parallel_driver_writes/customer && \
    go build -o parallel_driver_writes *.go && ls -la

# Stage 2: lightweight "release"
FROM docker.io/library/debian:bookworm-slim

# Create target directories
RUN mkdir -p /opt/antithesis/test/v1/basic
RUN mkdir -p /opt/antithesis/test/v1/intermediate

# Copy the instrumented binaries from the build image
COPY --from=builder \
    /commands/src/antithesis/commands-instrumented/basic/customer/singleton_driver_basic \
    /opt/antithesis/test/v1/basic/

COPY --from=builder \
    /commands/src/antithesis/commands-instrumented/intermediate/finally_consistent_data/customer/finally_consistent_data \
    /opt/antithesis/test/v1/intermediate/

COPY --from=builder \
    /commands/src/antithesis/commands-instrumented/intermediate/parallel_driver_writes/customer/parallel_driver_writes \
    /opt/antithesis/test/v1/intermediate/

# Copy symbols for each component (??? how does this work with multiple???)
COPY --from=builder \
    /commands/src/antithesis/commands-instrumented/basic/symbols/* \
    /symbols/

COPY --from=builder \
    /commands/src/antithesis/commands-instrumented/intermediate/finally_consistent_data/symbols/* \
    /symbols/

COPY --from=builder \
    /commands/src/antithesis/commands-instrumented/intermediate/parallel_driver_writes/symbols/* \
    /symbols/

# Make all executables runnable
RUN chmod +x /opt/antithesis/test/v1/basic/singleton_driver_basic \
    /opt/antithesis/test/v1/intermediate/finally_consistent_data \
    /opt/antithesis/test/v1/intermediate/parallel_driver_writes

ENTRYPOINT ["sleep", "infinity"]
