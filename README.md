# dev-switchboard

Local developer switchboard for routing `http://localhost:5173` to one active app.

## Commands

```sh
go run ./cmd/dev-switchboard serve
go run ./cmd/dev-switchboard add marketing 5174
go run ./cmd/dev-switchboard list
go run ./cmd/dev-switchboard activate marketing
go run ./cmd/dev-switchboard active
go run ./cmd/dev-switchboard remove marketing
```
