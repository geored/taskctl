# taskctl

A simple CLI task manager written in Go.

## Usage

```bash
taskctl add "Buy groceries" --priority high
taskctl list
taskctl done 1
taskctl delete 1
```

## Build

```bash
go build -o taskctl .
```

## Test

```bash
go test ./...
```
