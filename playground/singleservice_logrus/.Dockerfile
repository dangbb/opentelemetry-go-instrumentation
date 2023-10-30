FROM golang:1.20
WORKDIR /app
COPY service .
RUN go build -o singleservice_logrus main.go
ENTRYPOINT ["/app/main"]