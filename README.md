# dev-switchboard

Local developer switchboard for routing `http://localhost:5173` to one active app by default.

When `serve` is running, switchboard claims loopback port `5173` by default so Vite apps started with plain `pnpm dev` should typically fall through to `5174`, `5175`, and so on. Use `--port` to override the proxy port.

## Commands

```sh
go run ./cmd/dev-switchboard
go run ./cmd/dev-switchboard --help
go run ./cmd/dev-switchboard -h
go run ./cmd/dev-switchboard serve
go run ./cmd/dev-switchboard serve --port 6000
go run ./cmd/dev-switchboard serve -p 6000
go run ./cmd/dev-switchboard start
go run ./cmd/dev-switchboard start --port 6000
go run ./cmd/dev-switchboard start -p 6000
go run ./cmd/dev-switchboard status
go run ./cmd/dev-switchboard tui
go run ./cmd/dev-switchboard tui -p 6000
go run ./cmd/dev-switchboard stop
go run ./cmd/dev-switchboard add 5174
go run ./cmd/dev-switchboard add 5175 --name my-app
go run ./cmd/dev-switchboard list
go run ./cmd/dev-switchboard activate 5174
go run ./cmd/dev-switchboard activate my-app
go run ./cmd/dev-switchboard activate 5175 --name my-app
go run ./cmd/dev-switchboard active
go run ./cmd/dev-switchboard rename 5175 my-app
go run ./cmd/dev-switchboard remove my-app
```

Running without a command, or with `--help` / `-h`, prints the built-in help message with command descriptions and examples.
