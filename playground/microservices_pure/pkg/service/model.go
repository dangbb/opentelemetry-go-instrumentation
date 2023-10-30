package service

type EventType uint64

const (
	InterceptorInput EventType = 1
	WarehouseInsert  EventType = 2
)

type Audit struct {
	ServiceName string
	RequestType EventType
}

type Warehouse struct {
	Location string
	Name     string
}
