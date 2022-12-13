FROM golang:latest as build
WORKDIR /app

COPY go.mod ./
RUN go mod tidy

COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -installsuffix cgo -o main ./cmd/server

#FROM gcr.io/distroless/static
FROM ubuntu

RUN apt-get update && apt-get install -y \
curl

COPY --from=build /app/.  /

CMD ["/main"]