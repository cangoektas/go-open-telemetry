FROM golang:alpine as build
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY ./cmd/fib/*.go ./
RUN go build -o ./fib
RUN ls

FROM alpine
WORKDIR /app
COPY --from=build ./app/fib ./
CMD ["./fib"]
