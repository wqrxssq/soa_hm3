FROM golang:1.27-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates curl unzip
ARG TARGETARCH
ARG PROTOC_VERSION=29.3
RUN case "$TARGETARCH" in \
      amd64) ZIP=linux-x86_64 ;; \
      arm64) ZIP=linux-aarch_64 ;; \
      *) echo "unsupported $TARGETARCH"; exit 1 ;; \
    esac \
    && curl -fsSL -o /tmp/protoc.zip "https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-${ZIP}.zip" \
    && unzip -q /tmp/protoc.zip -d /usr/local \
    && rm /tmp/protoc.zip
COPY go.mod go.sum tools.go ./
RUN go mod download
COPY . .
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go google.golang.org/grpc/cmd/protoc-gen-go-grpc \
    && go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen
ENV PATH="/go/bin:${PATH}"
RUN mkdir -p internal/booking/api \
    && PROTO_INCLUDE=/usr/local/include sh scripts/protoc.sh \
    && oapi-codegen --config api/openapi/codegen.yaml -o internal/booking/api/openapi.gen.go api/openapi/openapi.yaml
RUN CGO_ENABLED=0 go test ./...
RUN CGO_ENABLED=0 go build -o /out/booking ./cmd/booking

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=build /out/booking /app/booking
USER nobody
EXPOSE 8080
ENTRYPOINT ["/app/booking"]
