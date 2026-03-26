FROM golang:1.24 AS builder

arg dev_mode=false
WORKDIR /workspace
# Copy go.mod and go.sum based on mode
COPY go.mod ./
RUN if [ "$dev_mode" = "true" ]; then \
      git clone --branch main --depth 1 https://github.com/kubevirt/kubevirt.git /kubevirt && \
      go mod edit -replace kubevirt.io/kubevirt=/kubevirt && \
      go mod edit -replace kubevirt.io/client-go=/kubevirt/staging/src/kubevirt.io/client-go && \
      go mod tidy; \
    else \
      go mod tidy; \
    fi

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o kubevirt-vm-to-pod ./cmd

# Final image
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/kubevirt-vm-to-pod .
USER 65532:65532

ENTRYPOINT ["/kubevirt-vm-to-pod"]
