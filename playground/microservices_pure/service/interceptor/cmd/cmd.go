package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/alecthomas/kong"
	jsoniter "github.com/json-iterator/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"microservice/config"
	pb "microservice/pb/proto"
	"microservice/pkg/service"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func logLogrus() {
	logrus.SetLevel(logrus.DebugLevel)

	logrus.Trace("Something very low level.")
	logrus.Debug("Useful debugging information.")
	logrus.Info("Something noteworthy happened!")
	logrus.Warn("You should probably take a look at this.")

	go func() {
		logrus.SetLevel(logrus.DebugLevel)

		logrus.Trace("Something very low level.")
		logrus.Debug("Useful debugging information.")
		logrus.Info("Something noteworthy happened!")
		logrus.Warn("You should probably take a look at this.")
	}()
}

func main() {
	cfg := config.Config{}
	kong.Parse(&cfg)

	logrus.SetLevel(logrus.DebugLevel)

	// craft audit service
	conn, err := grpc.Dial(cfg.AuditAddress, grpc.WithTransportCredentials(
		insecure.NewCredentials()))
	if err != nil {
		logrus.Fatalf("can establish grpc client conn %s", err.Error())
	}

	auditService := pb.NewAuditServiceClient(conn)
	defer conn.Close()

	// create gin-gonic server
	r := gin.New()
	r.POST("/send-package", func(c *gin.Context) {
		var warehouse service.Warehouse

		if err := c.ShouldBindJSON(&warehouse); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			logrus.Errorf("error when parsed %s", err.Error())
			return
		}

		requestBodyStr, err := jsoniter.Marshal(warehouse)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			logrus.Errorf("error when marshal %s", err.Error())
			return
		}

		logrus.Debugf("Receiver request: %s", requestBodyStr)

		// send request to warehouse service
		resp, err := http.Post(fmt.Sprintf("%s%s", cfg.WarehouseAddress, "/insert-warehouse"),
			"application/json",
			bytes.NewBuffer(requestBodyStr))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			logrus.Errorf("error when comm to warehouse service %s", err.Error())
			return
		}

		body, _ := io.ReadAll(resp.Body)
		logrus.Infof("Response Body:", string(body))

		if resp.Status != "200 OK" {
			c.JSON(http.StatusInternalServerError, gin.H{"message": body})
			logrus.Errorf("error at warehouse service %s", body)
			return
		}

		go logLogrus()

		// send to audit service
		grpcResponse, err := auditService.AuditSend(context.Background(), &pb.AuditSendRequest{
			ServiceName: "interceptor",
			RequestType: uint64(service.InterceptorInput),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			logrus.Errorf("error when comm to audit service %s", err.Error())
			return
		}

		if grpcResponse.Code != 200 {
			c.JSON(http.StatusInternalServerError, gin.H{"message": grpcResponse.Message})
			logrus.Errorf("error at audit service %s", grpcResponse.Message)
			return
		}

		// send to audit service 2
		grpcResponse, err = auditService.AuditSend(context.Background(), &pb.AuditSendRequest{
			ServiceName: "interceptor 2",
			RequestType: uint64(service.InterceptorInput),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			logrus.Errorf("error when comm to audit service %s", err.Error())
			return
		}

		if grpcResponse.Code != 200 {
			c.JSON(http.StatusInternalServerError, gin.H{"message": grpcResponse.Message})
			logrus.Errorf("error at audit service %s", grpcResponse.Message)
			return
		}

		// get from customer service
		resp, err = http.Get(fmt.Sprintf("%s%s", cfg.CustomerAddress, "/customer")) // TODO, change this to customer endpoint
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			logrus.Errorf("error when comm to customer service %s", err.Error())
			return
		}

		body, _ = io.ReadAll(resp.Body)
		logrus.Info("Response Body from customer service:", string(body))

		if resp.Status != "200 OK" {
			c.JSON(http.StatusInternalServerError, gin.H{"message": body})
			logrus.Errorf("error at customer service %s", body)
			return
		}

		// customer 2
		resp, err = http.Get(fmt.Sprintf("%s%s", cfg.CustomerAddress, "/customer")) // TODO, change this to customer endpoint
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			logrus.Errorf("error when comm to customer service %s", err.Error())
			return
		}

		body, _ = io.ReadAll(resp.Body)
		logrus.Info("Response Body from customer service:", string(body))

		if resp.Status != "200 OK" {
			c.JSON(http.StatusInternalServerError, gin.H{"message": body})
			logrus.Errorf("error at customer service %s", body)
			return
		}

		// return ok
		c.JSON(http.StatusOK, gin.H{"message": "OK"})
	})
	if err := r.Run(fmt.Sprintf("0.0.0.0:%d", 8090)); err != nil {
		logrus.Fatalf("cant run server %s", err.Error())
	}
}
