package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/asccclass/notebooklm-go/internal/auth"
	"github.com/asccclass/notebooklm-go/internal/config"
	"github.com/asccclass/notebooklm-go/internal/library"
	"github.com/asccclass/notebooklm-go/internal/mcp"
	"github.com/asccclass/notebooklm-go/internal/session"
	"github.com/asccclass/notebooklm-go/internal/tools"
	"github.com/asccclass/notebooklm-go/internal/utils"
)

const (
	serverName    = "notebooklm-mcp"
	serverVersion = "1.0.0"
)

func main() {
	utils.InitLogger(os.Getenv("DEBUG") == "1")

	if len(os.Args) > 1 && os.Args[1] == "config" {
		runConfigCLI(os.Args[2:])
		return
	}

	if err := runServer(); err != nil {
		slog.Error("Server error", "err", err)
		os.Exit(1)
	}
}

func runServer() error {
	cfg := config.Load(); 
	slog.Info("🚀 Starting NotebookLM MCP Server (Go)",
		"name", serverName, "version", serverVersion)

	authMgr := auth.New(&cfg)

	lib, err := library.New(&cfg)
	if err != nil {
		return fmt.Errorf("init library: %w", err)
	}

	sessionMgr := session.NewManager(&cfg, authMgr)
	defer sessionMgr.Close()

	settingsMgr := utils.NewSettingsManager(cfg.ConfigDir)
	allowedNames := settingsMgr.FilterTools(tools.AllToolNames)
	slog.Info("🔧 Tools enabled", "count", len(allowedNames))

	handler := tools.New(sessionMgr, authMgr, lib, &cfg, allowedNames)

	srv := mcp.New(mcp.ServerInfo{Name: serverName, Version: serverVersion}, handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		slog.Info("🛑 Shutting down...")
		cancel()
	}()

	return srv.Run(ctx)
}

func runConfigCLI(args []string) {
	cfg := config.Load(); 
	sm := utils.NewSettingsManager(cfg.ConfigDir)

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: notebooklm-mcp-go config <get|set|reset>")
		os.Exit(1)
	}

	switch args[0] {
	case "get":
		settings := sm.GetSettings()
		fmt.Printf("Profile:       %s\n", settings.Profile)
		fmt.Printf("DisabledTools: %s\n", strings.Join(settings.DisabledTools, ", "))

	case "set":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: config set <profile|disabled-tools> <value>")
			os.Exit(1)
		}
		key, value := args[1], args[2]
		switch key {
		case "profile":
			sm.SetProfile(utils.Profile(value))
		case "disabled-tools":
			sm.SetDisabledTools(strings.Split(value, ","))
		default:
			fmt.Fprintf(os.Stderr, "Unknown setting: %s\n", key)
			os.Exit(1)
		}
		if err := sm.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Save error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Updated %s = %s\n", key, value)

	case "reset":
		sm.Reset()
		if err := sm.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Save error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Settings reset to defaults")

	default:
		fmt.Fprintf(os.Stderr, "Unknown config command: %s\n", args[0])
		os.Exit(1)
	}
}
