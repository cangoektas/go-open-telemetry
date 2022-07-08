FROM golang:alpine as build
ARG name
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY ./cmd/$name/*.go ./
RUN go build -o ./run

FROM alpine
WORKDIR /app
COPY --from=build ./app/run ./
CMD ["./run"]
