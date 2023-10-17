FROM golang:1.20
WORKDIR /app
COPY . .
RUN go build -o warehouse service/warehouse/cmd/cmd.go
ENTRYPOINT ["/app/warehouse"]