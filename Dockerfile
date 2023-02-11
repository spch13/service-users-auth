FROM golang:1.18-alpine3.16 as builder
COPY . /src/
WORKDIR /src/
RUN go mod download
RUN go build -o ./.bin/app ./cmd/service/main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /src/.bin/app .
COPY --from=builder /src/.env .
COPY --from=builder /src/swaggerui ./swaggerui/

CMD [ "./app" ]