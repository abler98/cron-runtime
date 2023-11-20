# cron-runtime

Simple utility to run commands by a cron schedule. It is useful for having cron like functionality in Docker containers.

## Key features

* Extended cron format support (thanks to [robfig/cron](https://github.com/robfig/cron))
* Graceful shutdown with timeout
* Runs in foreground (useful for Docker containers)

## Installation

```bash
go install github.com/abler98/cron-runtime@latest
```

## Syntax

```bash
Usage: cron-runtime [options] <cron expression> -- program [parameters...]
```

### Options

* `-h`: Show help message
* `-once`: Run command once and exit
* `-kill-timeout <seconds>`: Timeout for killing the command (default: 0 - no timeout)
* `-debug`: Print debug messages

### Example

```bash
# print "Hello, world!" every 5 minutes
cron-runtime "*/5 * * * *" -- echo "Hello, world!"
```

## Usage in Docker

```dockerfile
FROM golang:1.21-alpine AS cron-runtime

RUN go install github.com/abler98/cron-runtime@latest

FROM my-laravel-app

COPY --from=cron-runtime /go/bin/cron-runtime /usr/bin/cron-runtime

ENTRYPOINT ["cron-runtime", "-kill-timeout=60", "* * * * *", "--", "php", "artisan", "schedule:run"]
```
