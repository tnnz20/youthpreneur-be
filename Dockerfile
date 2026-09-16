FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /app/web ./cmd/web

FROM alpine:3.21

WORKDIR /app

COPY --from=build /app/web /app/web

EXPOSE 8080

ENTRYPOINT ["/app/web"]
