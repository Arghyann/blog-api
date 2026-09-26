#!/usr/bin/env bash
set -e

FILE="$1"
if [ -z "$FILE" ] || [ ! -f "$FILE" ]; then
    echo "Usage: ./parse_md.sh <path-to-md-file> [output-json-path]"
    exit 1
fi

BASENAME=$(basename "$FILE" .md)

# Generate slug: lowercase, replace spaces and special characters with hyphens
SLUG=$(echo "$BASENAME" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/-/g' | sed -E 's/^-|-$//g')

# Title from filename (capitalizing words)
TITLE=$(echo "$BASENAME" | awk '{for(i=1;i<=NF;i++)sub(/./,toupper(substr($i,1,1)),$i)}1')

OUTPUT="${2:-body.json}"

jq -Rs --arg title "$TITLE" --arg slug "$SLUG" \
   '{title: $title, slug: $slug, body: .}' "$FILE" > "$OUTPUT"

# Remove the markdown file
rm "$FILE"

echo "Created $OUTPUT (slug: $SLUG) and removed $FILE"
