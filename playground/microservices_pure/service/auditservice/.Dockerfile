FROM golang:1.20
WORKDIR /app
COPY . .
RUN go build -o audit service/auditservice/cmd/main.go
ENTRYPOINT ["/app/audit"]