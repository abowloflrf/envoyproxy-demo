package main

import (
	"context"

	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/log"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/envoyproxy/go-control-plane/pkg/test/v3"
	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	cache := cachev3.NewSnapshotCache(false, cachev3.IDHash{}, log.LoggerFuncs{
		DebugFunc: func(s string, i ...interface{}) { logrus.Debugf(s, i...) },
		InfoFunc:  func(s string, i ...interface{}) { logrus.Infof(s, i...) },
		WarnFunc:  func(s string, i ...interface{}) { logrus.Warnf(s, i...) },
		ErrorFunc: func(s string, i ...interface{}) { logrus.Errorf(s, i...) },
	})
	snapshot := GenerateSnapshot()

	if err := snapshot.Consistent(); err != nil {
		logrus.Fatalf("snapshot consistent: %+v for %+v", err, snapshot)
	}
	logrus.Infof("will serve snapshot: %+v", snapshot)

	if err := cache.SetSnapshot("test-id", snapshot); err != nil {
		logrus.Fatalf("snapshot error: %+v for %+v", err, snapshot)
	}

	ctx := context.Background()
	srv := server.NewServer(ctx, cache, &test.Callbacks{Debug: true})
	RunServer(ctx, srv, 18000)
}
