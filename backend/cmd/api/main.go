// İPKS API — giriş noktası.
//
//	/app/api           HTTP sunucusunu başlatır
//	/app/api -seed     idempotent seed adımlarını uygular ve çıkar
package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ipks/ipks/backend/internal/config"
	"github.com/ipks/ipks/backend/internal/db"
	"github.com/ipks/ipks/backend/internal/logger"
	"github.com/ipks/ipks/backend/internal/seed"
	"github.com/ipks/ipks/backend/internal/server"
)

func main() {
	seedOnly := flag.Bool("seed", false, "seed adımlarını uygula ve çık")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logger.New(cfg.LogLevel, cfg.Env)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.DBDSN)
	if err != nil {
		log.Error("veritabanı bağlantısı kurulamadı", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if *seedOnly {
		opts := seed.Options{
			AdminEmail:    cfg.BootstrapAdminEmail,
			AdminPassword: cfg.BootstrapAdminPassword,
		}
		if err := seed.Apply(ctx, pool, log, opts); err != nil {
			log.Error("seed başarısız", "err", err)
			os.Exit(1)
		}
		log.Info("seed tamamlandı")
		return
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.New(cfg, pool, log),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("İPKS API dinliyor", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("sunucu hatası", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("kapanış sinyali alındı, graceful shutdown")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
