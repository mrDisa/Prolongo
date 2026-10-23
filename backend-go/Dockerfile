FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/core ./cmd/core

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/core /core
EXPOSE 8000
# distroless has no shell or wget: compose runs ["/core", "-healthcheck"]
ENTRYPOINT ["/core"]
