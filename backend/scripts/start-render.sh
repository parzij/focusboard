#!/bin/sh
set -eu

echo "Applying database migrations..."
/usr/local/bin/migrate up

echo "Starting FocusBoard API..."
exec /usr/local/bin/api
