package main

import (
	"github.com/gitu/mr-info/pkg/logging"
)

//go:generate go run github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen -package gitlab -generate types,client -o pkg/gitlab/gitlab.gen.go openapi-gitlab.yaml
func main() {
	logging.LogHandler.SetReportCaller(true)
	logging.Log.Info("Hello, world!")
}
