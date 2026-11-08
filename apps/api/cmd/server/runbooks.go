package main

import (
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/config"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/runbook"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/runbookservice"
)

func buildRunbookRuntime(settings config.Config,database databaseRuntime)(*runbookservice.Service,error){
	if settings.DemoDeployment{return nil,nil}
	if _,ok:=database.(interface{isFakeDatabase()});ok{return nil,nil}
	repository,ok:=database.(runbook.Repository);if !ok{return nil,errHTTPBoundary}
	_,key,err:=database.AuthDigestKey();defer clear(key);if err!=nil{return nil,errHTTPBoundary}
	service,err:=runbookservice.New(repository,key,time.Now);if err!=nil{return nil,errHTTPBoundary};return service,nil
}
