FROM golang:1.20
WORKDIR /app
COPY . .
RUN go build -o interceptor service/interceptor/cmd/cmd.go
ENTRYPOINT ["/app/interceptor"]