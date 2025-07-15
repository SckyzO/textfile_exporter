# 🚀 Textfile Exporter

[![GitHub Release](https://img.shields.io/github/v/release/SckyzO/textfile_exporter?style=flat-square)](https://github.com/SckyzO/textfile_exporter/releases)
[![Build Status](https://github.com/SckyzO/textfile_exporter/actions/workflows/release.yml/badge.svg?branch=main)](https://github.com/SckyzO/textfile_exporter/actions/workflows/release.yml)

A robust and flexible Prometheus exporter for metrics from textual files.

This exporter monitors a directory for `*.prom` files, parses the Prometheus metrics within them (including timestamps), and exposes them on an HTTP endpoint for scraping. It is ideal for capturing metrics from cron jobs, batch scripts, or any process that cannot expose its own metrics endpoint.

## 🤔 Why use this exporter over the node_exporter's textfile collector?

While the standard Prometheus `node_exporter` includes a [textfile collector](https://github.com/prometheus/node_exporter#textfile-collector-module), this project was created to address some of its limitations for more advanced use cases:

- **Timestamp Support**: The `node_exporter` does not support timestamps in metric files. All metrics are assigned the timestamp of the scrape, which is unsuitable for jobs that run offline (e.g., nightly backups). This exporter reads and applies the original timestamps from the `.prom` files, ensuring data accuracy.

- **In-Memory Cache & Expiration**: This exporter maintains an in-memory cache of metrics with a configurable expiration time. Metrics can persist for a while even after their source file is deleted, which is ideal for ephemeral or short-lived jobs. The `node_exporter`'s collector re-reads files on every scrape, so metrics disappear instantly with their files.

- **Flexible File Cleanup**: The `node_exporter` has very basic file management. This exporter allows you to run any external command on old files, giving you the flexibility to archive, compress, or log them as needed.

- **Standalone and Focused**: This is a lightweight, dedicated binary. If you only need to export metrics from text files, you can deploy this tool without the overhead of the full `node_exporter`.

## ✨ Features

- 📁 **Directory Monitoring**: Continuously scans a specified directory for `*.prom` files.
- ⏰ **Timestamp Support**: Natively supports timestamps in metric lines, ensuring accurate data timing.
- ⚙️ **Flexible Configuration**: All settings are configurable via command-line flags.
- 🧹 **Automatic Cleanup**: Can be configured to run a custom command on old metric files.
- 🏷️ **Dynamic Versioning**: Binaries are built with embedded version information (Git commit, branch, build date).

## 🚀 Getting Started

### 🏗️ Building from source

To build the exporter, you need a working Go environment. Then, use the provided `Makefile`:

```bash
make build
```

This will create the `textfile_exporter` binary in the root directory.

### 💡 Usage

Run the exporter by pointing it to the directory containing your `.prom` files:

```bash
./textfile_exporter --textfile.directory="/path/to/your/metrics"
```

You can then access the metrics at `http://localhost:9014/metrics`.

### 📈 Internal Metrics

The exporter also exposes its own internal metrics:

- `textfile_exporter_scanned_files_count`: The number of `.prom` files found during the last scan.
- `textfile_exporter_last_scan_timestamp`: Unix timestamp of the last successful scan.
- Standard Go process metrics (`process_*`) and Go runtime metrics (`go_*`).

### 📝 Examples

**Example `.prom` file:**

A `.prom` file contains standard Prometheus metrics, optionally with a timestamp at the end of the line (in milliseconds since epoch).

```bash
$ cat /path/to/your/metrics/backup-job.prom
```
```
# HELP backup_duration_seconds Duration of backup process in seconds.
# TYPE backup_duration_seconds gauge
backup_duration_seconds{host="server1"} 138 1698680700000
# HELP backup_copied_bytes Amount of data copied in bytes.
# TYPE backup_copied_bytes gauge
backup_copied_bytes{host="server1"} 37543832 1698680700000
```

**Scraping the metrics:**

You can use `curl` to check the metrics exposed by the exporter:

```bash
$ curl http://localhost:9014/metrics
```
```
# HELP backup_copied_bytes Amount of data copied in bytes.
# TYPE backup_copied_bytes gauge
backup_copied_bytes{host="server1"} 3.7543832e+07 1698680700000
# HELP backup_duration_seconds Duration of backup process in seconds.
# TYPE backup_duration_seconds gauge
backup_duration_seconds{host="server1"} 138 1698680700000
```

## ⚙️ Configuration

The exporter is configured via command-line flags:

| Flag                             | Description                                                                    | Default     |
| -------------------------------- | ------------------------------------------------------------------------------ | ----------- |
| `--web.listen-address`           | Address on which to expose metrics and web interface.                          | `:9014`     |
| `--textfile.directory`           | Path for prom file or directory of `*.prom` files.                             | `.`         |
| `--scan-interval`                | The interval at which to scan the directory for `.prom` files.                 | `30s`       |
| `--memory-max-age`               | Max age of in-memory metrics before they are garbage collected.                | `25h`       |
| `--[no-]files-min-age`         | Enable or disable the minimum age check for files.                             | `true`      |
| `--files-min-age-duration`     | Minimum age of files to be considered old, if `--files-min-age` is enabled.  | `6h`        |
| `--old-files-external-command`   | External command to execute on old files. The filename is passed as an argument. | `ls -l`     |
| `--web.config.file`              | Path for web configuration file (e.g., for TLS).                    | `""`        |
| `--scanner.recursive`            | Recursively scan for `.prom` files in the given directory.             | `false`     |

### 🔐 Web Configuration

The exporter's web server can be configured to use TLS, client certificate authentication, and basic authentication. All of these options are configured in a YAML file passed to the `--web.config.file` flag.

**Example `web-config.yml`:**

```yaml
tls_server_config:
  cert_file: /path/to/your/server-cert.pem
  key_file: /path/to/your/server-key.pem
  client_ca_file: /path/to/your/client-ca.pem

basic_auth:
  username: "myuser"
  password_file: "/path/to/password.txt"
```

#### TLS and Client Authentication

- `cert_file` and `key_file` are the server's TLS certificate and private key.
- `client_ca_file` is the certificate authority (CA) file used to validate client certificates. If provided, the exporter will require a valid client certificate for all connections.

#### Basic Authentication

- `username` is the username for basic authentication.
- `password_file` is the path to a file containing the password for basic authentication.

You would then run the exporter like this:

```bash
./textfile_exporter --web.config.file="web-config.yml"
```

The exporter will then serve metrics over HTTPS on the address specified by `--web.listen-address`, protected by any configured authentication methods.

### 🏷️ Version Information

To check the version of the binary, use the `--version` flag:

```bash
./textfile_exporter --version
```

## ⚙️ Deployment as a systemd Service

To run the exporter as a systemd service on a Linux system, follow these steps:

1.  **Create a dedicated user and group**: It is recommended to run the exporter with a non-privileged user.

    ```bash
    sudo useradd --no-create-home --shell /bin/false textfile_exporter
    ```

2.  **Create the metrics directory**: This is where your applications will drop their `.prom` files.

    ```bash
    sudo mkdir /var/lib/textfile_exporter
    sudo chown textfile_exporter:textfile_exporter /var/lib/textfile_exporter
    ```

3.  **Install the binary**: Copy the compiled `textfile_exporter` binary to a suitable location.

    ```bash
    sudo cp ./textfile_exporter /usr/local/bin/textfile_exporter
    ```

4.  **Install the service file**: Copy the provided service file to the systemd directory.

    ```bash
    sudo cp ./systemd-example/textfile_exporter.service /etc/systemd/system/textfile_exporter.service
    ```

5.  **Enable and start the service**:

    ```bash
    sudo systemctl daemon-reload
    sudo systemctl enable textfile_exporter
    sudo systemctl start textfile_exporter
    ```

    You can check the status of the service with `sudo systemctl status textfile_exporter`.

## 🔗 Fork Information

This repository is a fork of the original [IBM/textfile-exporter](https://github.com/IBM/textfile-exporter). It has been refactored to use a standard Go project layout, a more modern CLI interface with `kingpin`, and an automated release workflow via GitHub Actions.

## 📄 License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
