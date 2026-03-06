package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dev-switchboard/internal/app"
	"dev-switchboard/internal/control"
	"dev-switchboard/internal/service"
	tuiapp "dev-switchboard/internal/tui"
)

const defaultProxyPort = 5173

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return usageError()
	}

	client := control.NewClient(control.SocketPath())

	switch args[0] {
	case "serve", "daemon":
		port, err := parseServerCommand(args[1:])
		if err != nil {
			return err
		}
		return runServer(ctx, port)
	case "start":
		port, err := parseServerCommand(args[1:])
		if err != nil {
			return err
		}
		return startCommand(ctx, client, port)
	case "stop":
		return stopCommand(ctx, client)
	case "status":
		return statusCommand(ctx, client)
	case "tui":
		ownedServer, err := ensureServerRunning(ctx, client)
		if err != nil {
			return err
		}
		return tuiapp.Run(client, ownedServer)
	case "add":
		port, name, err := parsePortNameCommand("add", args[1:])
		if err != nil {
			return err
		}
		candidate, err := client.Add(ctx, port, name)
		if err != nil {
			return err
		}
		fmt.Printf("added %s\n", formatApp(candidate))
		return nil
	case "list":
		apps, activeName, err := client.List(ctx)
		if err != nil {
			return err
		}
		if len(apps) == 0 {
			fmt.Println("no apps registered")
			return nil
		}
		for _, candidate := range apps {
			marker := " "
			if candidate.Name == activeName {
				marker = "*"
			}
			fmt.Printf("%s %s\n", marker, formatApp(candidate))
		}
		return nil
	case "activate":
		target, name, err := parseActivateCommand(args[1:])
		if err != nil {
			return err
		}
		candidate, err := client.Activate(ctx, target, name)
		if err != nil {
			return err
		}
		fmt.Printf("active app: %s\n", formatApp(*candidate))
		return nil
	case "active":
		candidate, err := client.Active(ctx)
		if err != nil {
			return err
		}
		if candidate == nil {
			fmt.Println("none")
			return nil
		}
		fmt.Println(formatApp(*candidate))
		return nil
	case "rename":
		oldName, newName, err := parseRenameCommand(args[1:])
		if err != nil {
			return err
		}
		candidate, err := client.Rename(ctx, oldName, newName)
		if err != nil {
			return err
		}
		fmt.Printf("renamed %s\n", formatApp(candidate))
		return nil
	case "remove":
		if len(args) != 2 {
			return fmt.Errorf("usage: dev-switchboard remove <name>")
		}
		return client.Remove(ctx, args[1])
	default:
		return usageError()
	}
}

func runServer(ctx context.Context, port int) error {
	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	return service.Run(sigCtx, service.Config{
		SocketPath:       control.SocketPath(),
		ProxyListenAddrs: proxyListenAddrs(port),
		ReservedPort:     port,
	})
}

func startCommand(ctx context.Context, client *control.Client, port int) error {
	started, err := startDaemonIfNeeded(ctx, client, port)
	if err != nil {
		return err
	}
	if !started {
		fmt.Println("already running")
		return nil
	}
	fmt.Println("started")
	return nil
}

func stopCommand(ctx context.Context, client *control.Client) error {
	if err := client.Health(ctx); err != nil {
		fmt.Println("already stopped")
		return nil
	}
	if err := client.Shutdown(ctx); err != nil {
		return err
	}
	if err := waitForShutdown(client, 5*time.Second); err != nil {
		return err
	}
	fmt.Println("stopped")
	return nil
}

func statusCommand(ctx context.Context, client *control.Client) error {
	status, err := client.Status(ctx)
	if err != nil {
		fmt.Println("stopped")
		return nil
	}
	fmt.Println("running")
	fmt.Printf("pid: %d\n", status.PID)
	fmt.Printf("apps: %d\n", status.AppCount)
	if status.Active == nil {
		fmt.Println("active: none")
	} else {
		fmt.Printf("active: %s\n", formatApp(*status.Active))
	}
	if len(status.ProxyListenAddrs) > 0 {
		fmt.Printf("listen: %s\n", strings.Join(status.ProxyListenAddrs, ", "))
	}
	return nil
}

func ensureServerRunning(ctx context.Context, client *control.Client) (bool, error) {
	return startDaemonIfNeeded(ctx, client, defaultProxyPort)
}

func startDaemonIfNeeded(ctx context.Context, client *control.Client, port int) (bool, error) {
	if err := client.Health(ctx); err == nil {
		return false, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return false, err
	}
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return false, err
	}
	defer devNull.Close()

	cmd := exec.Command(executable, "daemon")
	if port != defaultProxyPort {
		cmd.Args = append(cmd.Args, "--port", strconv.Itoa(port))
	}
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	cmd.Stdin = devNull
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return false, err
	}
	_ = cmd.Process.Release()

	if err := waitForHealth(client, 5*time.Second); err != nil {
		return false, err
	}
	return true, nil
}

func waitForHealth(client *control.Client, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		err := client.Health(ctx)
		cancel()
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for switchboard to start")
}

func waitForShutdown(client *control.Client, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		err := client.Health(ctx)
		cancel()
		if err != nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for switchboard to stop")
}

func usageError() error {
	return fmt.Errorf("usage: dev-switchboard <serve|start|stop|status|tui|add|list|activate|active|rename|remove>")
}

func parseServerCommand(args []string) (int, error) {
	port := defaultProxyPort

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port":
			if i+1 >= len(args) {
				return 0, fmt.Errorf("usage: dev-switchboard <serve|daemon|start> [--port <port>]")
			}
			parsedPort, err := strconv.Atoi(args[i+1])
			if err != nil || parsedPort < 1 || parsedPort > 65535 {
				return 0, fmt.Errorf("invalid port: %s", args[i+1])
			}
			port = parsedPort
			i++
		default:
			return 0, fmt.Errorf("usage: dev-switchboard <serve|daemon|start> [--port <port>]")
		}
	}

	return port, nil
}

func parsePortNameCommand(command string, args []string) (int, string, error) {
	var (
		name    string
		portArg string
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name", "-n":
			if i+1 >= len(args) {
				return 0, "", fmt.Errorf("usage: dev-switchboard %s <port> [--name <name>]", command)
			}
			name = args[i+1]
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				return 0, "", fmt.Errorf("usage: dev-switchboard %s <port> [--name <name>]", command)
			}
			if portArg != "" {
				return 0, "", fmt.Errorf("usage: dev-switchboard %s <port> [--name <name>]", command)
			}
			portArg = args[i]
		}
	}

	if portArg == "" {
		return 0, "", fmt.Errorf("usage: dev-switchboard %s <port> [--name <name>]", command)
	}

	port, err := strconv.Atoi(portArg)
	if err != nil {
		return 0, "", fmt.Errorf("invalid port: %w", err)
	}

	return port, strings.TrimSpace(name), nil
}

func parseActivateCommand(args []string) (string, string, error) {
	var (
		name   string
		target string
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name", "-n":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("usage: dev-switchboard activate <port|name> [--name <name>]")
			}
			name = args[i+1]
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				return "", "", fmt.Errorf("usage: dev-switchboard activate <port|name> [--name <name>]")
			}
			if target != "" {
				return "", "", fmt.Errorf("usage: dev-switchboard activate <port|name> [--name <name>]")
			}
			target = args[i]
		}
	}

	if target == "" {
		return "", "", fmt.Errorf("usage: dev-switchboard activate <port|name> [--name <name>]")
	}
	if _, err := strconv.Atoi(target); err != nil && name != "" {
		return "", "", fmt.Errorf("usage: dev-switchboard activate <port|name> [--name <name>]")
	}

	return strings.TrimSpace(target), strings.TrimSpace(name), nil
}

func parseRenameCommand(args []string) (string, string, error) {
	if len(args) != 2 {
		return "", "", fmt.Errorf("usage: dev-switchboard rename <old-name> <new-name>")
	}

	return strings.TrimSpace(args[0]), strings.TrimSpace(args[1]), nil
}

func formatApp(candidate app.App) string {
	return fmt.Sprintf("%s -> %d", candidate.Name, candidate.Port)
}

func proxyListenAddrs(port int) []string {
	portValue := strconv.Itoa(port)
	return []string{
		net.JoinHostPort("127.0.0.1", portValue),
		net.JoinHostPort("::1", portValue),
	}
}
