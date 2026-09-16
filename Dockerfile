# Cross-compile from the build platform: far faster than emulating the target.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/request-logger .

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/request-logger /request-logger
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/request-logger"]
