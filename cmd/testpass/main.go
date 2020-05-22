package main

import (
	"context"
	"flag"
	"github.com/tmazeika/testpass/config"
	"github.com/tmazeika/testpass/repo"
	"github.com/tmazeika/testpass/rest"
	"github.com/tmazeika/testpass/seed"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
)

var (
	configFileFlag = flag.String("config", "config/config.json",
		"The path to the JSON configuration file")
	dbFlag = flag.String("db", "bunt",
		"The database implementation to use: bunt, mongo")
	helpFlag = flag.Bool("help", false,
		"Shows this help")
	seedFlag = flag.Bool("seed", false,
		"Whether to seed the database with test data")
)

func main() {
	flag.Parse()
	if *helpFlag {
		flag.PrintDefaults()
		return
	}

	cfg, err := config.New(*configFileFlag)
	if err != nil {
		log.Fatalln(err)
	}

	repos, err := repo.NewBuntRepos(cfg.Storage.Bunt.File, func() string {
		return primitive.NewObjectID().Hex()
	})
	if err != nil {
		log.Fatalf("create BuntDB repos: %v\n", err)
	}
	defer func() {
		if err := repos.Close(); err != nil {
			log.Printf("close BuntDB repos: %v\n", err)
		}
	}()

	if *seedFlag {
		log.Println("Seeding...")
		if err := seed.Seed(repos); err != nil {
			log.Fatalln(err)
		}
	} else {
		log.Println("Starting up...")
		listenHttp(cfg, repos)
	}
}

func listenHttp(cfg *config.Config, repos repo.Repos) {
	srv := http.Server{
		Addr:    net.JoinHostPort(cfg.Http.Host, strconv.Itoa(int(cfg.Http.Port))),
		Handler: rest.Handler(repos),
	}
	srvDone := make(chan interface{})

	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint
		log.Println("Shutting down...")

		ctx, _ := context.WithTimeout(context.Background(), cfg.Http.ShutdownTimeout)
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("shutdown http: %v\n", err)
		}
		close(srvDone)
	}()

	log.Println("Binding to", srv.Addr)
	err := srv.ListenAndServeTLS(cfg.Http.TlsCertFile, cfg.Http.TlsKeyFile)
	if err != http.ErrServerClosed {
		log.Fatalf("listen http: %v\n", err)
	}
	<-srvDone
}
