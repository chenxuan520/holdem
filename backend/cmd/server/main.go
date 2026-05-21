package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"holdem/backend/internal/config"
	"holdem/backend/internal/httpapi"
	"holdem/backend/internal/store"
)

func main() {
	runtimePath := resolveRuntimeConfigPath()
	runtimeConfig, err := config.LoadRuntimeConfig(runtimePath)
	if err != nil {
		log.Fatalf("load runtime config: %v", err)
	}

	presetsPath := config.ResolveRelativeToConfig(runtimePath, runtimeConfig.Backend.PresetsPath)
	presets, err := config.LoadPresets(presetsPath)
	if err != nil {
		log.Fatalf("load presets: %v", err)
	}

	dataPath := config.ResolveRelativeToConfig(runtimePath, runtimeConfig.Backend.DataPath)
	replayStore, err := store.NewSQLiteReplayStore(dataPath)
	if err != nil {
		log.Fatalf("open replay store: %v", err)
	}

	server := httpapi.NewServer(presets, replayStore)
	addr := fmt.Sprintf(":%d", runtimeConfig.Backend.Port)

	log.Printf("holdem backend listening on %s using %s", addr, presetsPath)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatal(err)
	}
}

func resolveRuntimeConfigPath() string {
	return config.ResolveRuntimeConfigPath([]string{
		filepath.Clean("config/app.json"),
		filepath.Clean("../config/app.json"),
	})
}
