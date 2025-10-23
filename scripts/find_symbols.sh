#!/bin/bash
# Helper script to find RPC-related symbols in a Go binary
# Usage: ./find_symbols.sh /path/to/binary

set -e

COLOR_BLUE='\033[0;34m'
COLOR_GREEN='\033[0;32m'
COLOR_RESET='\033[0m'

if [ $# -eq 0 ]; then
    echo "Usage: $0 <path-to-binary>"
    echo "Example: $0 /usr/local/bin/geth"
    exit 1
fi

BINARY="$1"

if [ ! -f "$BINARY" ]; then
    echo "Error: Binary not found: $BINARY"
    exit 1
fi

echo -e "${COLOR_BLUE}Searching for RPC-related symbols in: ${BINARY}${COLOR_RESET}"
echo ""

echo "=== JSON-RPC Server Symbols ==="
nm -D "$BINARY" 2>/dev/null | grep -i "rpc.*serve" || echo "None found"
echo ""

echo "=== JSON-RPC Handler Symbols ==="
nm -D "$BINARY" 2>/dev/null | grep -i "rpc.*handle" || echo "None found"
echo ""

echo "=== HTTP Handler Symbols ==="
nm -D "$BINARY" 2>/dev/null | grep -i "http.*serve" | head -20 || echo "None found"
echo ""

echo "=== Go-Ethereum RPC Symbols ==="
nm -D "$BINARY" 2>/dev/null | grep "github.com/ethereum/go-ethereum/rpc" | head -20 || echo "None found"
echo ""

echo "=== Method Call Symbols ==="
nm -D "$BINARY" 2>/dev/null | grep -i "method.*call" | head -10 || echo "None found"
echo ""

echo -e "${COLOR_GREEN}Tip: Use the full symbol path for TARGET_SYMBOL${COLOR_RESET}"
echo "Example: github.com/ethereum/go-ethereum/rpc.(*Server).serveRequest"
echo ""

# Try objdump as alternative
if command -v objdump &> /dev/null; then
    echo "=== Alternative: Using objdump ==="
    objdump -T "$BINARY" 2>/dev/null | grep -i rpc | grep -i serve | head -10 || echo "None found"
    echo ""
fi

# Check if it's a Go binary
if file "$BINARY" | grep -q "Go BuildID"; then
    echo -e "${COLOR_GREEN}✓ Confirmed: This is a Go binary${COLOR_RESET}"
else
    echo "⚠ Warning: This may not be a Go binary"
fi
