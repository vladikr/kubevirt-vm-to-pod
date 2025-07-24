FROM golang:1.21 AS builder

WORKDIR /workspace
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o kubevirt-vm-to-pod ./cmd

# Final image
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/kubevirt-vm-to-pod .
USER 65532:65532

ENTRYPOINT ["/kubevirt-vm-to-pod"]
