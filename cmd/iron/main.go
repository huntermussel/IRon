package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sync"

	"iron/internal/communicators"
	_ "iron/internal/communicators/signal"
	_ "iron/internal/communicators/slack"
	_ "iron/internal/communicators/telegram"
	_ "iron/internal/communicators/whatsapp"
	"iron/internal/gateway"
	"iron/internal/onboarding"

	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"
	cfgFile string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "iron",
		Short: "IRon — Personal AI Assistant. 🛡️",
		Long: `
██╗██████╗  ██████╗ ███╗   ██╗
██║██╔══██╗██╔═══██╗████╗  ██║
██║██████╔╝██║   ██║██╔██╗ ██║
██║██╔══██╗██║   ██║██║╚██╗██║
██║██║  ██║╚██████╔╝██║ ╚████║
╚═╝╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝

Personal AI Assistant in Go.
Version: ` + version,
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "~/.iron/config.json", "config file (default: ~/.iron/config.json)")

	rootCmd.AddCommand(
		chatCmd(),
		execCmd(),
		onboardCmd(),
		serveCmd(),
		versionCmd(),
		doctorCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func chatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "chat",
		Short: "Start the interactive chat session (default)",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create a gateway (which encapsulates the chat logic)
			gw := gateway.New(cfgFile)

			// Setup graceful shutdown
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				<-sigCh
				fmt.Println("\nReceived signal, shutting down...")
				cancel()
			}()

			return gw.Run(ctx)
		},
	}
}

func execCmd() *cobra.Command {
	var timeout int

	cmd := &cobra.Command{
		Use:   "exec [prompt]",
		Short: "Execute a single prompt and exit",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			gw := gateway.New(cfgFile)

			ctx := context.Background()
			var cancel context.CancelFunc
			if timeout > 0 {
				ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
			} else {
				ctx, cancel = context.WithCancel(ctx)
			}
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				cancel()
			}()

			return gw.Execute(ctx, args[0])
		},
	}

	cmd.Flags().IntVarP(&timeout, "timeout", "t", 300, "Execution timeout in seconds")
	return cmd
}

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start IRon as a background daemon listening to all configured communicators (Telegram, Slack, WhatsApp, Signal)",
		RunE: func(cmd *cobra.Command, args []string) error {
			gw := gateway.New(cfgFile)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				fmt.Println("\nReceived signal, shutting down communicators...")
				cancel()
			}()

			comms := communicators.All()
			if len(comms) == 0 {
				fmt.Println("No communicators registered.")
				return nil
			}

			fmt.Printf("Starting %d background communicators...\n", len(comms))
			var wg sync.WaitGroup
			for _, c := range comms {
				wg.Add(1)
				go func(comm communicators.Communicator) {
					defer wg.Done()
					if err := comm.Start(ctx, gw); err != nil {
						fmt.Fprintf(os.Stderr, "Error running communicator %s: %v\n", comm.ID(), err)
					}
				}(c)
			}

			wg.Wait()
			return nil
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("IRon %s\n", version)
		},
	}
}

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check configuration and dependencies",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Checking IRon health...")
			// TODO: Add real checks (Ollama connection, disk space, permissions)
			fmt.Println("✅ Environment seems OK (Placeholder)")
		},
	}
}

func onboardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "onboard",
		Short: "Run the interactive onboarding wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := onboarding.RunTUI(); err != nil {
				return fmt.Errorf("onboarding failed: %w", err)
			}
			return nil
		},
	}
}
