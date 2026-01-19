# pod-doctor

A CLI tool for diagnosing Kubernetes pod issues.

When a pod fails, debugging requires running multiple commands. **pod-doctor** runs all diagnostics in one command and provides actionable insights.

## Features

- **Interactive TUI** - Browse namespaces and pods with keyboard navigation
- **Status Analysis** - Detect CrashLoopBackOff, ImagePullBackOff, Pending, OOMKilled, etc.
- **Log Analysis** - Fetch logs, detect common errors (panic, exception, connection refused)
- **Event Timeline** - Show recent events related to the pod
- **Node Health** - Check if node has issues (disk pressure, memory pressure, not ready)
- **Recommendations** - Suggest fixes based on detected issues

## Installation

### Go

```bash
go install github.com/pavanInnamuri/pod-doctor@latest
```

### From Source

```bash
git clone https://github.com/pavanInnamuri/pod-doctor.git
cd pod-doctor
go build -o pod-doctor .
sudo mv pod-doctor /usr/local/bin/
```

## Usage

### Interactive TUI

Launch the interactive terminal UI:

```bash
pod-doctor
```

The TUI allows you to:
- Browse and select namespaces
- View pods with status, restarts, and age
- Filter pods by name
- Select a pod to run full diagnosis
- View issues and recommendations

### TUI Keys

| Key | Action |
|-----|--------|
| `↑` / `↓` / `k` / `j` | Navigate list |
| `Enter` | Select item |
| `/` | Start filtering |
| `Esc` | Cancel / Go back |
| `r` | Refresh |
| `q` | Quit |

### Diagnose a Pod

```bash
# Diagnose a specific pod
pod-doctor diagnose my-pod -n default

# Output as JSON
pod-doctor diagnose my-pod -o json
```

### Scan for Issues

```bash
# Scan all pods in a namespace
pod-doctor scan -n production

# Scan all namespaces
pod-doctor scan --all-namespaces

# Only show unhealthy pods
pod-doctor scan --unhealthy
```

## Example Output

```
Diagnosis: production/api-server-7d8f9c6b5-x2k4j

Status: ✗ CrashLoopBackOff
Node: worker-node-3 | Phase: Running | Age: 2h15m | Restarts: 47

Containers:
  • api-server: waiting (not ready, restarts: 47)
    Reason: CrashLoopBackOff

Issues Found: 2 critical, 1 warnings, 0 info

  ✗ Container api-server in CrashLoopBackOff
    Container is repeatedly crashing after starting
    restart_count: 47

  ✗ Container api-server was OOMKilled
    Container exceeded memory limit and was killed
    exit_code: 137

  ! [api-server] Connection refused
    Cannot connect to a service
    sample_match: dial tcp 10.0.0.5:5432: connection refused

Recommendations:
  1. Check container logs
     Review container logs to identify the crash cause
     $ kubectl logs api-server-7d8f9c6b5-x2k4j -n production --previous

  2. Increase memory limit
     Container exceeded memory limit; consider increasing it
     $ kubectl set resources deployment/<deployment-name> -c <container> --limits=memory=<new-limit>
```

## Commands

| Command | Description |
|---------|-------------|
| `pod-doctor` | Launch interactive TUI |
| `pod-doctor diagnose <pod>` | Diagnose a specific pod |
| `pod-doctor scan` | Scan pods for issues |
| `pod-doctor version` | Print version information |

## Flags

| Flag | Description |
|------|-------------|
| `--kubeconfig` | Path to kubeconfig file (default: ~/.kube/config) |
| `-n, --namespace` | Kubernetes namespace (default: default) |
| `-o, --output` | Output format: console, json, yaml |
| `-A, --all-namespaces` | Scan all namespaces |
| `--unhealthy` | Only show unhealthy pods |
| `-l, --selector` | Label selector to filter pods |

## License

MIT License
