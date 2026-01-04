# MatrixMigrate

A CLI tool for migrating from Mattermost to Matrix Synapse with multi-step, resumable migration support.

## Features

- **Multi-step Migration**: Migrate users, teams, channels, and memberships in organized steps
- **SSH Tunnel Support**: Securely connect to remote servers via SSH port forwarding
- **Beautiful TUI**: Interactive terminal UI powered by Bubble Tea
- **Multi-language Support**: English (default) and Turkish interfaces
- **Resumable**: Checkpoint-based migration that can be paused and resumed
- **Mapping Files**: Generates mapping files to track Mattermost → Matrix entity relationships

## Installation

```bash
go install github.com/aligundogdu/matrixmigrate/cmd/matrixmigrate@latest
```

Or build from source:

```bash
git clone https://github.com/aligundogdu/matrixmigrate.git
cd matrixmigrate
go build -o matrixmigrate ./cmd/matrixmigrate
```

## Configuration

1. Copy the example configuration:
   ```bash
   cp config.example.yaml config.yaml
   ```

2. Edit `config.yaml` with your server details

3. Set environment variables for sensitive data:
   ```bash
   export MM_DB_PASSWORD="your-mattermost-db-password"
   export MATRIX_ADMIN_TOKEN="your-matrix-admin-token"
   ```

## Usage

### Interactive Mode (TUI)

```bash
# Start with default language (English)
matrixmigrate

# Start with Turkish interface
matrixmigrate --lang tr
```

### Batch Mode

```bash
# Run specific steps
matrixmigrate export assets
matrixmigrate import assets
matrixmigrate export memberships
matrixmigrate import memberships

# Run with specific config
matrixmigrate --config ./config.yaml --batch export assets
```

### Test Connections

```bash
matrixmigrate test mattermost
matrixmigrate test matrix
```

### Check Status

```bash
matrixmigrate status
```

## Migration Steps

| Step | Command | Description |
|------|---------|-------------|
| 1a | `export assets` | Export users, teams, channels from Mattermost |
| 1b | `import assets` | Create users, spaces, rooms in Matrix |
| 2a | `export memberships` | Export team/channel memberships from Mattermost |
| 2b | `import memberships` | Apply memberships in Matrix |

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Local Machine                            │
│  ┌─────────────┐  ┌──────────┐  ┌─────────────────────────┐│
│  │ MatrixMigrate│  │ Config   │  │ Data Store              ││
│  │     CLI     │──│  YAML    │  │ - assets/*.json.gz      ││
│  └──────┬──────┘  └──────────┘  │ - mappings/*.json       ││
│         │                       │ - state.json            ││
│         │                       └─────────────────────────┘│
└─────────┼───────────────────────────────────────────────────┘
          │
    ┌─────┴─────┐
    │           │
    ▼           ▼
┌────────┐  ┌────────┐
│Mattermost│  │ Matrix │
│PostgreSQL│  │  API   │
│ (SSH)    │  │ (SSH)  │
└────────┘  └────────┘
```

## License

MIT License

