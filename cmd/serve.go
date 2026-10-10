package cmd

import (
	"MediaNinja/core/application"
	"MediaNinja/core/tasks"
	"MediaNinja/server"
	"context"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

var listenAddress string
var openBrowser bool
var desktopMode bool
var frontendDirectory string
var serveCmd = &cobra.Command{
	Use: "serve", Short: "Start the local Web UI and task API",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()
		if desktopMode {
			go func() { io.Copy(io.Discard, cmd.InOrStdin()); cancel() }()
		}
		if err := cfg.Validate(); err != nil {
			return err
		}
		output, err := filepath.Abs(cfg.OutputDir)
		if err != nil {
			return err
		}
		defaults := *cfg
		defaults.OutputDir = output
		if err := os.MkdirAll(output, 0755); err != nil {
			return err
		}
		host, _, err := net.SplitHostPort(listenAddress)
		if err != nil {
			return err
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("local server requires a loopback IP address")
		}
		listener, err := net.Listen("tcp", listenAddress)
		if err != nil {
			return err
		}
		defer listener.Close()
		manager := tasks.New(application.Execute)
		defer manager.Close()
		handler, err := server.New(manager, listener.Addr().String(), defaults)
		if err != nil {
			return err
		}
		if frontendDirectory != "" {
			if err := handler.SetFrontendDirectory(frontendDirectory); err != nil {
				return err
			}
		}
		httpServer := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
		address := "http://" + listener.Addr().String()
		if desktopMode {
			if err := json.NewEncoder(cmd.OutOrStdout()).Encode(struct {
				Protocol int    `json:"protocol"`
				URL      string `json:"url"`
			}{Protocol: 1, URL: address}); err != nil {
				return err
			}
		} else {
			fmt.Println("Web UI:", address)
			fmt.Println("API bearer token:", handler.Token())
		}
		if openBrowser {
			if err := launchBrowser(address); err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "Open the URL manually:", err)
			}
		}
		done := make(chan error, 1)
		go func() { done <- httpServer.Serve(listener) }()
		select {
		case err := <-done:
			if err != http.ErrServerClosed {
				return err
			}
			return nil
		case <-ctx.Done():
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := httpServer.Shutdown(ctx); err != nil {
				httpServer.Close()
			}
			return nil
		}
	},
}

func launchBrowser(address string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", address).Start()
	case "darwin":
		return exec.Command("open", address).Start()
	default:
		return exec.Command("xdg-open", address).Start()
	}
}
func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVar(&listenAddress, "listen", "127.0.0.1:0", "Loopback listen address (port 0 selects an available port)")
	serveCmd.Flags().BoolVar(&desktopMode, "desktop", false, "Desktop child mode: JSON startup and stdin lifetime control")
	serveCmd.Flags().StringVar(&frontendDirectory, "frontend-dir", "", "Serve an on-disk frontend build during development")
	serveCmd.Flags().BoolVar(&openBrowser, "open", false, "Open the Web UI in the default browser")
}
