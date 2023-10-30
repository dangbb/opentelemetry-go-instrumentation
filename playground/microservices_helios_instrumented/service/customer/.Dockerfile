FROM golang:1.20
WORKDIR /app
COPY . .
RUN go build -o customer service/customer/cmd/cmd.go
ENTRYPOINT ["/app/customer"]