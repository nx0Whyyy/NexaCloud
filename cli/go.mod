module github.com/nexastudio/nexacloud/cli

go 1.23

require (
	github.com/google/uuid v1.6.0
	github.com/spf13/cobra v1.8.0
	github.com/nexastudio/nexacloud/pkg/model v0.0.0
)

replace github.com/nexastudio/nexacloud/pkg/model => ../pkg/model
