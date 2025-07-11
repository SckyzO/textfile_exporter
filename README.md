# Textfile Exporter

A robust and flexible Prometheus exporter for metrics from textual files.

This exporter monitors a directory for `*.prom` files, parses the Prometheus metrics within them (including timestamps), and exposes them on an HTTP endpoint for scraping. It is ideal for capturing metrics from cron jobs, batch scripts, or any process that cannot expose its own metrics endpoint.

## Features

- **Directory Monitoring**: Continuously scans a specified directory for `*.prom` files.
- **Timestamp Support**: Natively supports timestamps in metric lines, ensuring accurate data timing.
- **Flexible Configuration**: All settings are configurable via command-line flags.
- **Automatic Cleanup**: Can be configured to run a custom command on old metric files.
- **Dynamic Versioning**: Binaries are built with embedded version information (Git commit, branch, build date).
- **Liveness Probe**: Includes a `/alive` endpoint for health checks.

## Getting Started

### Building from source

To build the exporter, you need a working Go environment. Then, use the provided `Makefile`:

```bash
make build
```

This will create the `textfile-exporter` binary in the root directory.

### Usage

Run the exporter by pointing it to the directory containing your `.prom` files:

```bash
./textfile-exporter --textfile.directory="/path/to/your/metrics"
```

You can then access the metrics at `http://localhost:9014/metrics`.

## Configuration

The exporter is configured via command-line flags:

| Flag                             | Description                                                                    | Default     |
| -------------------------------- | ------------------------------------------------------------------------------ | ----------- |
| `--web.listen-address`           | Address on which to expose metrics and web interface.                          | `:9014`     |
| `--textfile.directory`           | Path for prom file or directory of `*.prom` files.                             | `.`         |
| `--scan-interval`                | The interval at which to scan the directory for `.prom` files.                 | `30s`       |
| `--memory-max-age`               | Max age of in-memory metrics before they are garbage collected.                | `25h`       |
| `--files-min-age`                | Minimum age of a file to be considered old for cleanup.                        | `6h`        |
| `--old-files-external-command`   | External command to execute on old files. The filename is passed as an argument. | `ls -l`     |
| `--web.config.file`              | *[NOT IMPLEMENTED]* Path for web configuration (TLS, auth).                    | `""`        |

### Version Information

To check the version of the binary, use the `--version` flag:

```bash
./textfile-exporter --version
```

## Deployment as a systemd Service

To run the exporter as a systemd service on a Linux system, follow these steps:

1.  **Create a dedicated user and group**: It is recommended to run the exporter with a non-privileged user.

    ```bash
    sudo useradd --no-create-home --shell /bin/false textfile-exporter
    ```

2.  **Create the metrics directory**: This is where your applications will drop their `.prom` files.

    ```bash
    sudo mkdir /var/lib/textfile-exporter
    sudo chown textfile-exporter:textfile-exporter /var/lib/textfile-exporter
    ```

3.  **Install the binary**: Copy the compiled `textfile-exporter` binary to a suitable location.

    ```bash
    sudo cp ./textfile-exporter /usr/local/bin/textfile-exporter
    ```

4.  **Install the service file**: Copy the provided service file to the systemd directory.

    ```bash
    sudo cp ./systemd-example/textfile-exporter.service /etc/systemd/system/textfile-exporter.service
    ```

5.  **Enable and start the service**:

    ```bash
    sudo systemctl daemon-reload
    sudo systemctl enable textfile-exporter
    sudo systemctl start textfile-exporter
    ```

    You can check the status of the service with `sudo systemctl status textfile-exporter`.

## Fork Information

This repository is a fork of the original [IBM/textfile-exporter](https://github.com/IBM/textfile-exporter). It has been refactored to use a standard Go project layout, a more modern CLI interface with `kingpin`, and an automated release workflow via GitHub Actions.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.