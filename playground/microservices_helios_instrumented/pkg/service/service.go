package service

import (
	"context"
	logrus "github.com/helios/go-sdk/proxy-libs/helioslogrus"
	"gorm.io/gorm"
)

type AuditService interface {
	CreateAudit(context.Context, Audit) error
}

type auditService struct {
	db *gorm.DB
}

func NewAuditService(db *gorm.DB) AuditService {
	return &auditService{db}
}

func (s *auditService) CreateAudit(ctx context.Context, a Audit) error {
	if err := s.db.Model(&Audit{}).Create(a).Error; err != nil {
		logrus.Errorf("error create audit %s", err.Error())
		return err
	}

	return nil
}
