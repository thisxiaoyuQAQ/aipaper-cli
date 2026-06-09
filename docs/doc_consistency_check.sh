#!/bin/bash
# Documentation consistency check for module 20
# Validates: commands, export file names, env vars, README links

set -e
PASS=0
FAIL=0
TOTAL=0

check() {
    TOTAL=$((TOTAL + 1))
    if "$@" >/dev/null 2>&1; then
        PASS=$((PASS + 1))
        echo "  ✅ $1"
    else
        FAIL=$((FAIL + 1))
        echo "  ❌ $1"
    fi
}

echo "=== Documentation Consistency Check ==="
echo ""

# 1. CLI commands match implementation
echo "1. CLI commands in user-guide match internal/cli/cli.go"
for cmd in init status recover config; do
    TOTAL=$((TOTAL + 1))
    if grep -q "aipaper-cli $cmd" docs/user-guide.md && grep -q "\"$cmd\"" internal/cli/cli.go; then
        PASS=$((PASS + 1))
        echo "  ✅ Command '$cmd' in user-guide and cli.go"
    else
        FAIL=$((FAIL + 1))
        echo "  ❌ Command '$cmd' mismatch"
    fi
done

# 2. Export file names match internal/export/export.go constants
echo ""
echo "2. Export file names in user-guide match export.go constants"
for path in "final/paper.md" "final/paper.docx" "final/references.md" "final/citation-trace.json" "final/report.md"; do
    TOTAL=$((TOTAL + 1))
    if grep -q "$path" docs/user-guide.md && grep -q "\"$path\"" internal/export/export.go; then
        PASS=$((PASS + 1))
        echo "  ✅ Path '$path' in user-guide and export.go"
    else
        FAIL=$((FAIL + 1))
        echo "  ❌ Path '$path' mismatch"
    fi
done

# 3. .env.example variables explained in user-guide
echo ""
echo "3. Environment variables from .env.example in user-guide"
while IFS= read -r line; do
    # Extract variable name (skip comments and blank lines)
    var=$(echo "$line" | grep -oP '^[A-Z_]+(?==)' || true)
    if [ -n "$var" ]; then
        TOTAL=$((TOTAL + 1))
        if grep -q "$var" docs/user-guide.md; then
            PASS=$((PASS + 1))
            echo "  ✅ Variable '$var' documented"
        else
            FAIL=$((FAIL + 1))
            echo "  ❌ Variable '$var' NOT documented"
        fi
    fi
done < .env.example

# 4. README links point to existing files
echo ""
echo "4. README links point to existing files"
for link in "docs/user-guide.md" "docs/需求与架构.md" "docs/interfaces/_index.md" "docs/开发进度.md"; do
    TOTAL=$((TOTAL + 1))
    if [ -f "$link" ]; then
        PASS=$((PASS + 1))
        echo "  ✅ Link '$link' exists"
    else
        FAIL=$((FAIL + 1))
        echo "  ❌ Link '$link' NOT found"
    fi
done

# 5. Default models match TUI interface spec
echo ""
echo "5. Default models in user-guide match TUI spec"
for model in "gpt-5.5" "claude-opus-4-8" "llama3"; do
    TOTAL=$((TOTAL + 1))
    if grep -q "$model" docs/user-guide.md && grep -q "$model" docs/interfaces/tui.md; then
        PASS=$((PASS + 1))
        echo "  ✅ Model '$model' consistent"
    else
        FAIL=$((FAIL + 1))
        echo "  ❌ Model '$model' mismatch"
    fi
done

# 6. Non-goal capabilities not promised
echo ""
echo "6. Non-goal capabilities not promised in user-guide"
for term in "Web UI" "自动 API 测试" "手动选择部分材料"; do
    TOTAL=$((TOTAL + 1))
    # These should NOT appear as promised features
    if ! grep -q "$term" docs/user-guide.md; then
        PASS=$((PASS + 1))
        echo "  ✅ '$term' not promised (correct)"
    else
        FAIL=$((FAIL + 1))
        echo "  ❌ '$term' found in user-guide (should not be promised)"
    fi
done

echo ""
echo "=== Results ==="
echo "Total: $TOTAL | Pass: $PASS | Fail: $FAIL"

if [ $FAIL -gt 0 ]; then
    echo "❌ SOME CHECKS FAILED"
    exit 1
else
    echo "✅ ALL CHECKS PASSED"
    exit 0
fi
