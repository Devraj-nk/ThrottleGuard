FROM golang:1.23 AS build

WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /throttleguard ./cmd/throttleguard

FROM gcr.io/distroless/static-debian12
COPY --from=build /throttleguard /throttleguard
EXPOSE 8080
ENTRYPOINT ["/throttleguard"]
