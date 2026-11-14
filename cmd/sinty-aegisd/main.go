// sinty-aegisd is the public alert and emergency call service of SintyOS. It
// follows the official alert feeds and earthquake catalogues for the place the
// computer is in and publishes what matters on dev.sinty.Aegis1.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/godbus/dbus/v5"

	"github.com/singularityos-lab/sinty-aegis/internal/config"
	"github.com/singularityos-lab/sinty-aegis/internal/service"
)

func main() {
	path := flag.String("config", config.Path(), "configuration file")
	flag.Parse()
	log.SetFlags(0)
	session, err := dbus.ConnectSessionBus()
	if err != nil {
		log.Fatalf("session bus: %v", err)
	}
	system, err := dbus.ConnectSystemBus()
	if err != nil {
		log.Printf("system bus: %v", err)
		system = nil
	}
	engine, err := service.Serve(session, system, *path)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	engine.Run(ctx)
}
