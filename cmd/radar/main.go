package main

import (
	"context"
	"go-radar/internal/config"
	radardb "go-radar/internal/db"
	radarhttp "go-radar/internal/http"
	"go-radar/internal/scheduler"
	"log"
)

func main() {
	settings, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := radardb.Open(settings)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	goScheduler := scheduler.New(db, settings.EnableScheduler)
	goScheduler.Start(context.Background())
	defer goScheduler.Stop()

	router := radarhttp.NewRouterWithScheduler(settings, db, goScheduler)
	addr := ":" + settings.Port
	log.Printf("Go Radar listening on http://127.0.0.1%s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
