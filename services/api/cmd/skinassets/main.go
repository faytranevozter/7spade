package main

import (
	"bytes"
	"context"
	_ "embed"
	"log"
	"time"

	"github.com/faytranevozter/7spade/services/api/internal/config"
	"github.com/faytranevozter/7spade/services/api/internal/storage"
)

//go:embed assets/skins/backgrounds/gilded-table.svg
var gildedTable []byte

//go:embed assets/skins/frames/gold-spade.svg
var goldSpadeFrame []byte

//go:embed assets/skins/display-pictures/ace-spade.svg
var aceSpadePicture []byte

//go:embed assets/skins/player-card-backgrounds/gilded-seat.svg
var gildedSeat []byte

func main() {
	cfg := config.Load()
	client, err := storage.New(cfg.S3Config)
	if err != nil {
		log.Fatal(err)
	}

	assets := map[string][]byte{
		"skins/backgrounds/gilded-table.svg":            gildedTable,
		"skins/frames/gold-spade.svg":                   goldSpadeFrame,
		"skins/display-pictures/ace-spade.svg":          aceSpadePicture,
		"skins/player-card-backgrounds/gilded-seat.svg": gildedSeat,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := client.ConfigurePublicReadCORS(ctx, cfg.CORSAllowedOrigins); err != nil {
		log.Printf("could not configure bucket CORS; using service-worker opaque caching instead: %v", err)
	} else {
		log.Printf("configured public read CORS for %d origin(s)", len(cfg.CORSAllowedOrigins))
	}
	for key, asset := range assets {
		if err := client.PutObject(ctx, key, "image/svg+xml", "public, max-age=31536000, immutable", bytes.NewReader(asset)); err != nil {
			log.Fatal(err)
		}
		log.Printf("uploaded %s", key)
	}
}
