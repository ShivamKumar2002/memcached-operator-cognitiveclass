# Build the custom-scorecard-tests binary
FROM golang:1.23 AS builder
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY . .

# Build
RUN GOOS=linux GOARCH=$TARGETARCH go build .

# Final image.
FROM registry.access.redhat.com/ubi9/ubi-minimal:9.5

ENV HOME=/opt/custom-scorecard \
    USER_NAME=custom-scorecard \
    USER_UID=1001

RUN echo "${USER_NAME}:x:${USER_UID}:0:${USER_NAME} user:${HOME}:/sbin/nologin" >> /etc/passwd

WORKDIR ${HOME}

COPY --from=builder /workspace/custom-scorecard /usr/local/bin/custom-scorecard

ENTRYPOINT ["/usr/local/bin/custom-scorecard"]

USER ${USER_UID}