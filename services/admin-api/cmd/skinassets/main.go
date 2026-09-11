package main

import (
	"bytes"
	"context"
	_ "embed"
	"log"
	"time"

	"os"
	"strings"

	"github.com/faytranevozter/7spade/services/admin-api/internal/storage"
	"github.com/joho/godotenv"
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
	_ = godotenv.Load()
	region := os.Getenv("S3_REGION")
	if region == "" {
		region = "us-east-1"
	}
	client, err := storage.NewSigner(os.Getenv("S3_ENDPOINT"), region, os.Getenv("S3_BUCKET"), os.Getenv("S3_ACCESS_KEY_ID"), os.Getenv("S3_SECRET_ACCESS_KEY"), os.Getenv("S3_PUBLIC_URL"), os.Getenv("S3_USE_PATH_STYLE") == "true")
	if err != nil {
		log.Fatal(err)
	}
	if client == nil {
		log.Fatal("S3 storage is not configured")
	}

	assets := map[string][]byte{
		"skins/backgrounds/gilded-table.svg":            gildedTable,
		"skins/frames/gold-spade.svg":                   goldSpadeFrame,
		"skins/display-pictures/ace-spade.svg":          aceSpadePicture,
		"skins/player-card-backgrounds/gilded-seat.svg": gildedSeat,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	origins := []string{}
	for _, origin := range strings.Split(os.Getenv("SKIN_ASSET_CORS_ALLOWED_ORIGINS"), ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			origins = append(origins, origin)
		}
	}
	if err := client.ConfigurePublicReadCORS(ctx, origins); err != nil {
		log.Printf("could not configure bucket CORS; using service-worker opaque caching instead: %v", err)
	} else {
		log.Printf("configured public read CORS for %d origin(s)", len(origins))
	}
	for key, asset := range assets {
		if err := client.PutObjectWithCacheControl(ctx, key, "image/svg+xml", int64(len(asset)), "public, max-age=31536000, immutable", bytes.NewReader(asset)); err != nil {
			log.Fatal(err)
		}
		log.Printf("uploaded %s", key)
	}
}
