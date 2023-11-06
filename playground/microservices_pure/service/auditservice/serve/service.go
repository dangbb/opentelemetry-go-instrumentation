package serve

import (
	"context"
	"fmt"
	"google.golang.org/grpc/metadata"
	"net"

	_ "github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"microservice/config"
	pb "microservice/pb/proto"
	"microservice/pkg/service"
)

type AuditServer struct {
	pb.UnimplementedAuditServiceServer

	db service.AuditService
}

func (s *AuditServer) AuditSend(ctx context.Context, in *pb.AuditSendRequest) (*pb.AuditSendResponse, error) {
	logrus.Info("Receive audit")

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		logrus.Warnf("Cannot parse metadata")
	} else {
		traceheader := md.Get("traceparent")
		if len(traceheader) > 0 {
			logrus.Infof("Value of traceparent: %s", traceheader[0])
		} else {
			logrus.Warnf("Traceparent not found")
		}
	}

	// send to mysql
	if err := s.db.CreateAudit(ctx, service.Audit{
		ServiceName: in.ServiceName,
		RequestType: service.EventType(in.RequestType),
	}); err != nil {
		logrus.Infof("Error when write audit %s", err.Error())
		return nil, err
	}

	logrus.Info("Done stored audit")

	return &pb.AuditSendResponse{
		Code:    200,
		Message: "OK",
	}, nil
}

func RunAuditServer(cfg config.Config) {
	dns := cfg.MySqlConfig.GetDsn()
	db, err := gorm.Open(mysql.Open(dns), &gorm.Config{
		Logger:                                   logger.Default,
		DisableForeignKeyConstraintWhenMigrating: true,
	})

	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", 8091))
	if err != nil {
		logrus.Fatalf("fail to listen to port %d: %s\n", cfg.GrpcPort, err)
	}

	s := grpc.NewServer()
	pb.RegisterAuditServiceServer(s, &AuditServer{
		db: service.NewAuditService(db),
	})
	logrus.Info(fmt.Sprintf("Server listening at %v", lis.Addr()))
	if err := s.Serve(lis); err != nil {
		logrus.Fatalf("failed to serve: %s", err.Error())
	}
}
