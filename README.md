# switchbrd

Local developer `switchbrd` for routing `http://localhost:5173` to one active app by default.

When `serve` is running, `switchbrd` claims loopback port `5173` by default so Vite apps started with plain `pnpm dev` should typically fall through to `5174`, `5175`, and so on. Use `--port` to override the proxy port.

## Commands

```sh
go run ./cmd/switchbrd
go run ./cmd/switchbrd --help
go run ./cmd/switchbrd -h
go run ./cmd/switchbrd serve
go run ./cmd/switchbrd serve --port 6000
go run ./cmd/switchbrd serve -p 6000
go run ./cmd/switchbrd start
go run ./cmd/switchbrd start --port 6000
go run ./cmd/switchbrd start -p 6000
go run ./cmd/switchbrd status
go run ./cmd/switchbrd tui
go run ./cmd/switchbrd tui -p 6000
go run ./cmd/switchbrd stop
go run ./cmd/switchbrd add 5174
go run ./cmd/switchbrd add 5175 --name my-app
go run ./cmd/switchbrd list
go run ./cmd/switchbrd activate 5174
go run ./cmd/switchbrd activate my-app
go run ./cmd/switchbrd activate 5175 --name my-app
go run ./cmd/switchbrd active
go run ./cmd/switchbrd rename 5175 my-app
go run ./cmd/switchbrd remove my-app
```

Running without a command launches the terminal UI. Use `--help` or `-h` to print the built-in help message.
