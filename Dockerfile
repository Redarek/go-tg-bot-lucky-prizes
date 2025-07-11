FROM golang:1.24-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o bot ./cmd/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=build /app/bot .

CMD ["./bot"]
