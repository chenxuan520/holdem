package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
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

	// HOLDEM_AUTH_PASSWORD wins over the value in app.json. The repo-tracked
	// app.json defaults to empty, so committing accidentally never leaks a
	// real password; the env var is the recommended way to enable auth on
	// a personal machine without touching tracked files.
	authPassword := runtimeConfig.Auth.Password
	if env := os.Getenv("HOLDEM_AUTH_PASSWORD"); env != "" {
		authPassword = env
	}

	server := httpapi.NewServer(presets, replayStore, authPassword)
	addr := fmt.Sprintf(":%d", runtimeConfig.Backend.Port)

	authState := "disabled"
	if authPassword != "" {
		authState = "enabled"
	}
	log.Printf("holdem backend listening on %s using %s (auth: %s)", addr, presetsPath, authState)
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
