module github.com/nexastudio/nexacloud/services/scheduler

go 1.25

require (
	github.com/google/uuid v1.6.0
	github.com/nats-io/nats.go v1.38.0
	github.com/nexastudio/nexacloud/pkg/model v0.0.0
	gorm.io/gorm v1.25.12
	gorm.io/driver/postgres v1.5.11
)

replace github.com/nexastudio/nexacloud/pkg/model => ../../pkg/model
